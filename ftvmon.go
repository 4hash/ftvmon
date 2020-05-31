package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hpcloud/tail"
	"github.com/shirou/gopsutil/host"
	tb "gopkg.in/tucnak/telebot.v2"
)

//import "net/http"
//import _ "net/http/pprof"

var wg sync.WaitGroup
var mutex sync.Mutex

type subscriber string

type Monitor struct {
	Token           string
	Authorized      []string
	TonPath         string
	KeysPath        string
	Logfiles        []Logfile
	Checks          map[string]*Metric
	ExtChecks       map[string]*Metric
	subscribersFile string
	subscribers     []string
	bot             *tb.Bot
	prQueue         chan string
	hostname        string
	wg              sync.WaitGroup
}

type Metric struct {
	Enabled   bool
	Threshold float64
	Path      string
	Dev       string
	Name      string
	sync.Mutex
	message       string
	msgStatus     string
	adnlChanged   bool
	lastBlockTime int64
	lastState     bool
}

type Logfile struct {
	Enabled bool
	File    string
	Events  []LogEvent
}

type LogEvent struct {
	Enabled    bool
	Match      string
	IsRegex    bool
	MessageOn  string
	MessageOff string
	Threshold  int //number of events with Match in log, during Window, to trigger sending MessageOn
	Window     int //minutes, if 0 - trigger MessageOn every time the event occurs (no MessageOff)
	IncludeRaw bool
	re         *regexp.Regexp
	sync.Mutex
	events     []logRecord
	lastState  bool
	eventQueue chan string
}

type logRecord struct {
	raw     string
	eventTS time.Time
}

//to satisfy the interface
func (s subscriber) Recipient() string {
	return fmt.Sprintf("%s", s)
}

func find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

func sliceToFile(slice []string, file string) (err error) {
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	datawriter := bufio.NewWriter(f)
	for _, data := range slice {
		_, _ = datawriter.WriteString(data + "\n")
	}
	datawriter.Flush()
	f.Close()
	return
}

func (entry *LogEvent) isThresholdReached() bool {
	if (len(entry.events) > 0) && (len(entry.events) >= entry.Threshold) {
		return true
	}
	if len(entry.events) > 0 && entry.Window == 0 {
		return true
	}
	return false
}

func (monitor *Monitor) logWorker(entry *LogEvent) {
	defer wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			//log.Printf("now: %s", now.Format(time.RFC3339))
			limitTS := now.Add(time.Duration(-entry.Window) * time.Minute)
			//log.Printf("limitTS: %s", limitTS.Format(time.RFC3339))
			var freshEvents []logRecord
			//needs optimization
			for _, e := range entry.events {
				if e.eventTS.After(limitTS) {
					freshEvents = append(freshEvents, e)
				}
			}
			//log.Printf("%s events: %d", entry.Match, len(freshEvents))
			entry.events = make([]logRecord, len(freshEvents))
			copy(entry.events, freshEvents)
			currentState := entry.isThresholdReached()
			if !currentState && entry.lastState {
				monitor.prQueue <- fmt.Sprintf("LOGS: %s", entry.MessageOff)
				entry.lastState = false
			}
		case raw := <-entry.eventQueue:
			event := logRecord{raw, time.Now()}
			entry.events = append(entry.events, event)
			currentState := entry.isThresholdReached()
			if currentState && !entry.lastState {
				if entry.IncludeRaw {
					monitor.prQueue <- fmt.Sprintf("LOGS: %s: %s", entry.MessageOn, raw)
				} else {
					monitor.prQueue <- fmt.Sprintf("LOGS: %s", entry.MessageOn)
				}
				if entry.Window > 0 {
					entry.lastState = true
				}
			}
		}
	}
}

func (monitor *Monitor) msgDispatcher() {
	defer wg.Done()
	for {
		select {
		case message, ok := <-monitor.prQueue:
			if !ok {
				return
			}
			mutex.Lock()
			buf := make([]string, len(monitor.subscribers))
			copy(buf, monitor.subscribers)
			mutex.Unlock()
			if len(buf) > 0 {
				for _, recipient := range buf {
					monitor.bot.Send(subscriber(recipient), monitor.hostname+": "+message)
				}
			}
		}
	}
}

func (monitor *Monitor) subscribe(user *tb.User) (err error) {
	_, found := find(monitor.Authorized, user.Username)
	if !found {
		err = fmt.Errorf("User %s is not authorized", user.Username)
		return
	}
	log.Printf("Found authorized user in conf: %+s\n", user.Username)
	_, found = find(monitor.subscribers, fmt.Sprintf("%d", user.ID))
	if !found {
		log.Printf("Not subscribed yet: %d\n", user.ID)
		mutex.Lock()
		monitor.subscribers = append(monitor.subscribers, fmt.Sprintf("%d", user.ID))
		mutex.Unlock()
		err = sliceToFile(monitor.subscribers, monitor.subscribersFile)
		if err != nil {
			log.Println("Error creating subscribers file:", err)
		}
	} else {
		log.Printf("User already subscribed: %d\n", user.ID)
	}
	return
}

func (monitor *Monitor) status(user *tb.User) (err error) {
	_, found := find(monitor.Authorized, user.Username)
	if !found {
		err = fmt.Errorf("User %s is not authorized", user.Username)
		return
	}
	for _, metric := range monitor.Checks {
		metric.Lock()
		msg := metric.msgStatus
		metric.Unlock()
		if msg != "" {
			monitor.bot.Send(user, monitor.hostname+": "+msg)
		}
	}
	for _, metric := range monitor.ExtChecks {
		metric.Lock()
		msg := metric.msgStatus
		metric.Unlock()
		if msg != "" {
			monitor.bot.Send(user, monitor.hostname+": "+msg)
		}
	}
	return
}

func (monitor *Monitor) tailLog(logfile *Logfile) {
	defer wg.Done()
	defer log.Printf("Exiting a goroutine for %s...\n", logfile.File)
	t, err := tail.TailFile(logfile.File, tail.Config{
		Follow:    true,
		ReOpen:    true,
		MustExist: true,
		Location:  &tail.SeekInfo{Whence: 2},
	})
	if err != nil {
		log.Println("Error: ", err)
		return
	}
	for {
		select {
		case line := <-t.Lines:
			for n, _ := range logfile.Events {
				if logfile.Events[n].Enabled == true {
					if logfile.Events[n].IsRegex {
						if logfile.Events[n].re.MatchString(line.Text) {
							log.Printf("%s event in the log %s: %s\n", logfile.Events[n].Match, logfile.File, line.Text)
							logfile.Events[n].eventQueue <- line.Text
						}
					} else {
						if strings.Contains(line.Text, logfile.Events[n].Match) {
							log.Printf("%s event in the log %s: %s\n", logfile.Events[n].Match, logfile.File, line.Text)
							logfile.Events[n].eventQueue <- line.Text
						}
					}
				}
			}
		}
	}
}

func (Monitor *Monitor) checker() {
	defer wg.Done()
	// ExtChecks - every 60 seconds, Checks - continuously
	ticker := time.NewTicker(60 * time.Second)
	fcheck := func(checks map[string]*Metric) {
		for n, m := range checks {
			name := n
			metric := m
			if metric.Enabled {
				f := func() {
					v := reflect.ValueOf(Monitor)
					response := v.MethodByName(name).Call(nil)
					status, _ := response[0].Interface().(bool)
					err, _ := response[1].Interface().(error)
					if err != nil {
						log.Println("Error: ", err)
					}
					metric.Lock()
					if status != metric.lastState {
						//if status changed, metric.message is not empty ""
						log.Println(Monitor.hostname + ": " + metric.message)
						Monitor.prQueue <- metric.message
						metric.lastState = !metric.lastState
					}
					metric.Unlock()
				}
				Monitor.wg.Add(1)
				go f()
			}
		}
		//wait until all checks are complete
		Monitor.wg.Wait()
	}
	//Running ExtChecks for the first time without waiting
	fcheck(Monitor.ExtChecks)
	for {
		select {
		default:
			fcheck(Monitor.Checks)
		case <-ticker.C:
			fcheck(Monitor.ExtChecks)
		}
	}
}

func main() {
	//go func() {
	//	log.Println(http.ListenAndServe("10.1.1.16:6060", nil))
	//}()
	var monitor Monitor
	//cleanup
	//_ = os.Remove("current")
	//_ = os.Remove("previous")
	infostat, err := host.Info()
	if err != nil {
		log.Println("Error: ", err)
	}
	monitor.hostname = infostat.Hostname
	monitor.subscribersFile = "subscribers"
	monitor.prQueue = make(chan string, 100)
	configFile, err := os.Open("conf.json")
	if err != nil {
		log.Println("No config file: ", err)
		return
	}
	defer configFile.Close()
	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&monitor)
	if err != nil {
		log.Println("Error decoding config file: ", err)
		return
	}
	if monitor.TonPath != "" {
		monitor.TonPath = filepath.Clean(monitor.TonPath)
	}
	if monitor.KeysPath != "" {
		monitor.KeysPath = filepath.Clean(monitor.KeysPath)
	}
	sFile, err := os.Open(monitor.subscribersFile)
	if err != nil {
		log.Println("No subscribers yet, use /subscribe")
	} else {
		fileScanner := bufio.NewScanner(sFile)
		fileScanner.Split(bufio.ScanLines)
		for fileScanner.Scan() {
			monitor.subscribers = append(monitor.subscribers, fileScanner.Text())
		}
		sFile.Close()
		for _, eachline := range monitor.subscribers {
			log.Printf("Found a subscriber with ID: %s\n", eachline)
		}
	}
	monitor.bot, err = tb.NewBot(tb.Settings{
		Token:  monitor.Token,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Println("Error creating monitor: ", err)
		return
	}
	monitor.bot.Handle("/subscribe", func(m *tb.Message) {
		err := monitor.subscribe(m.Sender)
		if err != nil {
			log.Println("Error subscribing: ", err)
			monitor.bot.Send(m.Sender, "Not authorized.")
		} else {
			monitor.bot.Send(m.Sender, "Subscribed to updates!")
		}
		for _, eachline := range monitor.subscribers {
			log.Printf("Found a subscriber with ID: %s\n", eachline)
		}
	})
	monitor.bot.Handle("/status", func(m *tb.Message) {
		err := monitor.status(m.Sender)
		if err != nil {
			log.Println("Error sending status: ", err)
		}
	})
	wg.Add(1)
	go monitor.bot.Start()
	for i, l := range monitor.Logfiles {
		if l.Enabled {
		Label:
			for k, e := range l.Events {
				if e.Enabled {
					if e.IsRegex {
						log.Printf("Compiling regex %s...\n", monitor.Logfiles[i].Events[k].Match)
						monitor.Logfiles[i].Events[k].re, err = regexp.Compile(monitor.Logfiles[i].Events[k].Match)
						if err != nil {
							log.Printf("Failed to compile regex %s, removed the event from checking \n", monitor.Logfiles[i].Events[k].Match)
							monitor.Logfiles[i].Events[k].Enabled = false
							continue Label
						}
					}
					monitor.Logfiles[i].Events[k].Lock()
					monitor.Logfiles[i].Events[k].eventQueue = make(chan string)
					monitor.Logfiles[i].Events[k].Unlock()
					wg.Add(1)
					go monitor.logWorker(&monitor.Logfiles[i].Events[k])
				}
			}
			log.Printf("Launching a goroutine for %s...\n", l.File)
			wg.Add(1)
			go monitor.tailLog(&monitor.Logfiles[i])
		}
	}
	wg.Add(1)
	go monitor.msgDispatcher()
	wg.Add(1)
	go monitor.checker()
	wg.Wait()
}
