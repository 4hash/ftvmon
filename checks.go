package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
	//"github.com/shirou/gopsutil/process"
)

const checksInterval int = 5
const extChecksInterval int = 60

func (monitor *Monitor) CPU() {
	exit := func() {
		monitor.Checks["CPU"].Lock()
		monitor.Checks["CPU"].status = true
		monitor.Checks["CPU"].message = fmt.Sprintf("CPU: Can't get CPU Load")
		monitor.Checks["CPU"].msgStatus = fmt.Sprintf("CPU: Can't get CPU Load")
		monitor.Checks["CPU"].Unlock()
	}
	for {
		res, err := cpu.Percent(time.Duration(checksInterval)*time.Second, false)
		if err != nil {
			log.Println(err)
			exit()
			return
		}
		result := res[0]
		monitor.Checks["CPU"].Lock()
		if result >= monitor.Checks["CPU"].Threshold {
			monitor.Checks["CPU"].status = true
		} else {
			monitor.Checks["CPU"].status = false
		}
		if monitor.Checks["CPU"].status {
			monitor.Checks["CPU"].message = fmt.Sprintf("CPU: ALERT: CPU load %.2f%% is too high, over %.2f%% threshold", result, monitor.Checks["CPU"].Threshold)
		} else {
			monitor.Checks["CPU"].message = fmt.Sprintf("CPU: CPU load %.2f%% is back to normal, less than %.2f%% threshold", result, monitor.Checks["CPU"].Threshold)
		}
		monitor.Checks["CPU"].msgStatus = fmt.Sprintf("CPU: Current CPU load (all CPUs) is %.2f%%", result)
		monitor.Checks["CPU"].Unlock()
	}
}

func (monitor *Monitor) Mem() {
	exit := func() {
		monitor.Checks["Mem"].Lock()
		monitor.Checks["Mem"].status = true
		monitor.Checks["Mem"].message = fmt.Sprintf("MEM: Can't get memory usage")
		monitor.Checks["Mem"].msgStatus = fmt.Sprintf("MEM: Can't get memory usage")
		monitor.Checks["Mem"].Unlock()
	}
	ticker := time.NewTicker(time.Duration(checksInterval) * time.Second)
	for {
		select {
		case <-ticker.C:
			memstat, err := mem.VirtualMemory()
			if err != nil {
				log.Println(err)
				exit()
				return
			}
			result := memstat.UsedPercent
			total := float64(memstat.Total) / 1024 / 1024
			available := float64(memstat.Available) / 1024 / 1024
			used := float64(memstat.Used) / 1024 / 1024
			monitor.Checks["Mem"].Lock()
			if result >= monitor.Checks["Mem"].Threshold {
				monitor.Checks["Mem"].status = true
			} else {
				monitor.Checks["Mem"].status = false
			}
			if monitor.Checks["Mem"].status {
				monitor.Checks["Mem"].message = fmt.Sprintf("MEM: ALERT: Memory usage %.2f%% is too high, over %.2f%% threshold", result, monitor.Checks["Mem"].Threshold)
			} else {
				monitor.Checks["Mem"].message = fmt.Sprintf("MEM: Memory usage %.2f%% is back to normal, less than %.2f%% threshold", result, monitor.Checks["Mem"].Threshold)
			}
			monitor.Checks["Mem"].msgStatus = fmt.Sprintf("MEM: Memory %.0f Mb total, %.0f Mb available, %.0f Mb used, %.2f%% used", total, available, used, result)
			monitor.Checks["Mem"].Unlock()
		}
	}
}

func (monitor *Monitor) DiskSpace() {
	exit := func() {
		monitor.Checks["DiskSpace"].Lock()
		monitor.Checks["DiskSpace"].status = true
		monitor.Checks["DiskSpace"].message = fmt.Sprintf("DISK: Can't get disk space usage")
		monitor.Checks["DiskSpace"].msgStatus = fmt.Sprintf("DISK: Can't get disk space usage")
		monitor.Checks["DiskSpace"].Unlock()
	}
	ticker := time.NewTicker(time.Duration(checksInterval) * time.Second)
	for {
		select {
		case <-ticker.C:
			usage, err := disk.Usage(monitor.Checks["DiskSpace"].Path)
			if err != nil {
				log.Printf("Error in Diskspace, please check if %s exists", monitor.Checks["DiskSpace"].Path)
				log.Println(err)
				exit()
				return
			}
			result := usage.UsedPercent
			total := float64(usage.Total) / (1024 * 1024 * 1024)
			free := float64(usage.Free) / (1024 * 1024 * 1024)
			used := float64(usage.Used) / (1024 * 1024 * 1024)
			monitor.Checks["DiskSpace"].Lock()
			if result >= monitor.Checks["DiskSpace"].Threshold {
				monitor.Checks["DiskSpace"].status = true
			} else {
				monitor.Checks["DiskSpace"].status = false
			}
			if monitor.Checks["DiskSpace"].status {
				monitor.Checks["DiskSpace"].message = fmt.Sprintf("DISK: ALERT: Running low on free disk space, disk space usage is %.2f%% at %s", result, monitor.Checks["DiskSpace"].Path)
			} else {
				monitor.Checks["DiskSpace"].message = fmt.Sprintf("DISK: Disk space usage %.2f%% at %s is back to normal", result, monitor.Checks["DiskSpace"].Path)
			}
			monitor.Checks["DiskSpace"].msgStatus = fmt.Sprintf("DISK: Disk space at %s %.2f Gb total, %.2f Gb free, %.2f Gb used (%.2f%% used)", monitor.Checks["DiskSpace"].Path, total, free, used, result)
			monitor.Checks["DiskSpace"].Unlock()
		}
	}
}

func (monitor *Monitor) DiskIOPS() {
	exit := func() {
		monitor.Checks["DiskIOPS"].Lock()
		monitor.Checks["DiskIOPS"].status = true
		monitor.Checks["DiskIOPS"].message = fmt.Sprintf("DISK: Can't get disk IOPS")
		monitor.Checks["DiskIOPS"].msgStatus = fmt.Sprintf("DISK: Can't get disk IOPS")
		monitor.Checks["DiskIOPS"].Unlock()
	}
	const interval time.Duration = time.Duration(checksInterval) * time.Second
	for {
		counters_before, err := disk.IOCounters(monitor.Checks["DiskIOPS"].Dev)
		if err != nil {
			log.Println(err)
			exit()
			return
		}
		time.Sleep(interval)
		counters_after, err := disk.IOCounters(monitor.Checks["DiskIOPS"].Dev)
		if err != nil {
			log.Println(err)
			exit()
			return
		}
		result := float64(counters_after[monitor.Checks["DiskIOPS"].Dev].ReadCount+counters_after[monitor.Checks["DiskIOPS"].Dev].WriteCount-counters_before[monitor.Checks["DiskIOPS"].Dev].ReadCount-counters_before[monitor.Checks["DiskIOPS"].Dev].WriteCount) / float64(checksInterval)
		reads := float64(counters_after[monitor.Checks["DiskIOPS"].Dev].ReadCount-counters_before[monitor.Checks["DiskIOPS"].Dev].ReadCount) / float64(checksInterval)
		writes := float64(counters_after[monitor.Checks["DiskIOPS"].Dev].WriteCount-counters_before[monitor.Checks["DiskIOPS"].Dev].WriteCount) / float64(checksInterval)
		monitor.Checks["DiskIOPS"].Lock()
		if result >= monitor.Checks["DiskIOPS"].Threshold {
			monitor.Checks["DiskIOPS"].status = true
		} else {
			monitor.Checks["DiskIOPS"].status = false
		}
		if monitor.Checks["DiskIOPS"].status {
			monitor.Checks["DiskIOPS"].message = fmt.Sprintf("DISK: ALERT: Aggregate (reads + writes) %.2f IOPS on /dev/%s are too high, over %.2f IOPS threshold", result, monitor.Checks["DiskIOPS"].Dev, monitor.Checks["DiskIOPS"].Threshold)
		} else {
			monitor.Checks["DiskIOPS"].message = fmt.Sprintf("DISK: Aggregate (reads + writes) %.2f IOPS on /dev/%s are back to normal, less than %.2f IOPS threshold", result, monitor.Checks["DiskIOPS"].Dev, monitor.Checks["DiskIOPS"].Threshold)
		}
		monitor.Checks["DiskIOPS"].msgStatus = fmt.Sprintf("DISK: /dev/%s %.2f IOPS reads, %.2f IOPS writes, %.2f IOPS total", monitor.Checks["DiskIOPS"].Dev, reads, writes, result)
		monitor.Checks["DiskIOPS"].Unlock()
	}
}

func (monitor *Monitor) DiskIOUtil() {
	exit := func() {
		monitor.Checks["DiskIOUtil"].Lock()
		monitor.Checks["DiskIOUtil"].status = true
		monitor.Checks["DiskIOUtil"].message = fmt.Sprintf("DISK: Can't get disk IO utilisation")
		monitor.Checks["DiskIOUtil"].msgStatus = fmt.Sprintf("DISK: Can't get disk IO utilisation")
		monitor.Checks["DiskIOUtil"].Unlock()
	}
	const interval time.Duration = time.Duration(checksInterval) * time.Second
	for {
		counters_before, err := disk.IOCounters(monitor.Checks["DiskIOUtil"].Dev)
		if err != nil {
			log.Println(err)
			exit()
			return
		}
		time.Sleep(interval)
		counters_after, err := disk.IOCounters(monitor.Checks["DiskIOUtil"].Dev)
		if err != nil {
			log.Println(err)
			exit()
			return
		}
		result := float64(counters_after[monitor.Checks["DiskIOUtil"].Dev].WeightedIO-counters_before[monitor.Checks["DiskIOUtil"].Dev].WeightedIO) / ((float64(checksInterval) * 1000) / 100)
		monitor.Checks["DiskIOUtil"].Lock()
		if result >= monitor.Checks["DiskIOUtil"].Threshold {
			monitor.Checks["DiskIOUtil"].status = true
		} else {
			monitor.Checks["DiskIOUtil"].status = false
		}
		if monitor.Checks["DiskIOUtil"].status {
			monitor.Checks["DiskIOUtil"].message = fmt.Sprintf("DISK: ALERT: Disk IO utilisation %.2f%% on /dev/%s is too high, over %.2f%% threshold", result, monitor.Checks["DiskIOUtil"].Dev, monitor.Checks["DiskIOUtil"].Threshold)
		} else {
			monitor.Checks["DiskIOUtil"].message = fmt.Sprintf("DISK: Disk IO utilisation %.2f%% on /dev/%s is back to normal, less than %.2f%% threshold", result, monitor.Checks["DiskIOUtil"].Dev, monitor.Checks["DiskIOUtil"].Threshold)
		}
		monitor.Checks["DiskIOUtil"].msgStatus = fmt.Sprintf("DISK: /dev/%s disk IO utilisation is %.2f%%", monitor.Checks["DiskIOUtil"].Dev, result)
		monitor.Checks["DiskIOUtil"].Unlock()
	}
}

func (monitor *Monitor) DiskMBps() {
	exit := func() {
		monitor.Checks["DiskMBps"].Lock()
		monitor.Checks["DiskMBps"].status = true
		monitor.Checks["DiskMBps"].message = fmt.Sprintf("DISK: Can't get disk Mb/s")
		monitor.Checks["DiskMBps"].msgStatus = fmt.Sprintf("DISK: Can't get disk Mb/s")
		monitor.Checks["DiskMBps"].Unlock()
	}
	const interval time.Duration = time.Duration(checksInterval) * time.Second
	for {
		counters_before, err := disk.IOCounters(monitor.Checks["DiskMBps"].Dev)
		if err != nil {
			log.Println(err)
			exit()
			return
		}
		time.Sleep(interval)
		counters_after, err := disk.IOCounters(monitor.Checks["DiskMBps"].Dev)
		if err != nil {
			log.Println(err)
			exit()
			return
		}
		result := float64(counters_after[monitor.Checks["DiskMBps"].Dev].ReadBytes+counters_after[monitor.Checks["DiskMBps"].Dev].WriteBytes-counters_before[monitor.Checks["DiskMBps"].Dev].ReadBytes-counters_before[monitor.Checks["DiskMBps"].Dev].WriteBytes) / (float64(checksInterval) * 1024 * 1024)
		reads := float64(counters_after[monitor.Checks["DiskMBps"].Dev].ReadBytes-counters_before[monitor.Checks["DiskMBps"].Dev].ReadBytes) / (float64(checksInterval) * 1024 * 1024)
		writes := float64(counters_after[monitor.Checks["DiskMBps"].Dev].WriteBytes-counters_before[monitor.Checks["DiskMBps"].Dev].WriteBytes) / (float64(checksInterval) * 1024 * 1024)
		monitor.Checks["DiskMBps"].Lock()
		if result >= monitor.Checks["DiskMBps"].Threshold {
			monitor.Checks["DiskMBps"].status = true
		} else {
			monitor.Checks["DiskMBps"].status = false
		}
		if monitor.Checks["DiskMBps"].status {
			monitor.Checks["DiskMBps"].message = fmt.Sprintf("DISK: ALERT: Aggregate (reads + writes) %.2f Mb/s on /dev/%s is too high, over %.2f Mb/s threshold", result, monitor.Checks["DiskMBps"].Dev, monitor.Checks["DiskMBps"].Threshold)
		} else {
			monitor.Checks["DiskMBps"].message = fmt.Sprintf("DISK: Aggregate (reads + writes) %.2f Mb/s on /dev/%s is back to normal, less than %.2f Mb/s threshold", result, monitor.Checks["DiskMBps"].Dev, monitor.Checks["DiskMBps"].Threshold)
		}
		monitor.Checks["DiskMBps"].msgStatus = fmt.Sprintf("DISK: /dev/%s %.2f Mb/s reads, %.2f Mb/s writes, %.2f Mb/s total", monitor.Checks["DiskMBps"].Dev, reads, writes, result)
		monitor.Checks["DiskMBps"].Unlock()
	}
}

func (monitor *Monitor) NetMbs() {
	exit := func() {
		monitor.Checks["NetMbs"].Lock()
		monitor.Checks["NetMbs"].status = true
		monitor.Checks["NetMbs"].message = fmt.Sprintf("NET: Can't get network Mb/s")
		monitor.Checks["NetMbs"].msgStatus = fmt.Sprintf("NET: Can't get network Mb/s")
		monitor.Checks["NetMbs"].Unlock()
	}
	const interval time.Duration = time.Duration(checksInterval) * time.Second
	for {
		counters_before, err := net.IOCounters(false)
		if err != nil {
			log.Println(err)
			exit()
			return
		}
		time.Sleep(interval)
		counters_after, err := net.IOCounters(false)
		if err != nil {
			log.Println(err)
			exit()
			return
		}
		result := float64(counters_after[0].BytesSent+counters_after[0].BytesRecv-counters_before[0].BytesSent-counters_before[0].BytesRecv) / (float64(checksInterval) * 1024 * 1024)
		sent := float64(counters_after[0].BytesSent-counters_before[0].BytesSent) / (float64(checksInterval) * 1024 * 1024)
		received := float64(counters_after[0].BytesRecv-counters_before[0].BytesRecv) / (float64(checksInterval) * 1024 * 1024)
		monitor.Checks["NetMbs"].Lock()
		if result >= monitor.Checks["NetMbs"].Threshold {
			monitor.Checks["NetMbs"].status = true
		} else {
			monitor.Checks["NetMbs"].status = false
		}
		if monitor.Checks["NetMbs"].status {
			monitor.Checks["NetMbs"].message = fmt.Sprintf("NET: ALERT: Aggregate network (all interfaces) %.2f Mb/s is too high, over %.2f Mb/s threshold", result, monitor.Checks["NetMbs"].Threshold)
		} else {
			monitor.Checks["NetMbs"].message = fmt.Sprintf("NET: Aggregate network (all interfaces) %.2f Mb/s is back to normal, less than %.2f Mb/s threshold", result, monitor.Checks["NetMbs"].Threshold)
		}
		monitor.Checks["NetMbs"].msgStatus = fmt.Sprintf("NET: Network %.2f Mb/s outgoing traffic, %.2f Mb/s incoming traffic, %.2f Mb/s total", sent, received, result)
		monitor.Checks["NetMbs"].Unlock()
	}
}

/*
func (monitor *Monitor) Process() {
	exit := func() {
		monitor.Checks["Process"].Lock()
		monitor.Checks["Process"].status = true
		monitor.Checks["Process"].message = fmt.Sprintf("PROCESS: Can't get processes list")
		monitor.Checks["Process"].msgStatus = fmt.Sprintf("PROCESS: Can't get processes list")
		monitor.Checks["Process"].Unlock()
	}
	ticker := time.NewTicker(time.Duration(checksInterval) * time.Second)
	for {
		select {
		case <-ticker.C:
			procs, err := process.Processes()
			if err != nil {
				log.Println(err)
				exit()
				return
			}
			lookFor := monitor.Checks["Process"].Name
			monitor.Checks["Process"].Lock()
			monitor.Checks["Process"].status = true
			for _, pr := range procs {
				name, _ := pr.Exe()
				if strings.Contains(name, lookFor) {
					monitor.Checks["Process"].status = false
					break
				}
			}
			//Process running (no problems): status = false
			if monitor.Checks["Process"].status {
				monitor.Checks["Process"].message = fmt.Sprintf("PROCESS: ALERT: Process %s is not found", monitor.Checks["Process"].Name)
				monitor.Checks["Process"].msgStatus = fmt.Sprintf("PROCESS: ALERT: Process %s is not found", monitor.Checks["Process"].Name)
			} else {
				monitor.Checks["Process"].message = fmt.Sprintf("PROCESS: Process %s is running", monitor.Checks["Process"].Name)
				monitor.Checks["Process"].msgStatus = fmt.Sprintf("PROCESS: Process %s is running", monitor.Checks["Process"].Name)
			}
			monitor.Checks["Process"].Unlock()
		}
	}
}
*/

func (monitor *Monitor) Process() {
	exit := func() {
		monitor.ExtChecks["Process"].Lock()
		monitor.ExtChecks["Process"].status = true
		monitor.ExtChecks["Process"].message = fmt.Sprintf("PROCESS: Can't get processes' list")
		monitor.ExtChecks["Process"].msgStatus = fmt.Sprintf("PROCESS: Can't get processes' list")
		monitor.ExtChecks["Process"].Unlock()
	}
	ticker := time.NewTicker(time.Duration(extChecksInterval) * time.Second)
	f := func() {
		select {
		case <-ticker.C:
		}
	}
	for ; true; f() {
		var out bytes.Buffer
		cmd := exec.Command("ps", "-eLf")
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			err = fmt.Errorf("Error running ps to check process: %s", err)
			log.Println(err)
			exit()
			return
		}
		scanner := bufio.NewScanner(&out)
		monitor.ExtChecks["Process"].Lock()
		monitor.ExtChecks["Process"].status = true
		var i int = 0
		for scanner.Scan() {
			s := scanner.Text()
			if strings.Contains(s, monitor.ExtChecks["Process"].Name) {
				monitor.ExtChecks["Process"].status = false
				i++
			}
		}
		//Process running (no problems): status = false
		if monitor.ExtChecks["Process"].status {
			monitor.ExtChecks["Process"].message = fmt.Sprintf("PROCESS: ALERT: Process %s is not found", monitor.ExtChecks["Process"].Name)
			monitor.ExtChecks["Process"].msgStatus = fmt.Sprintf("PROCESS: ALERT: Process %s is not found", monitor.ExtChecks["Process"].Name)
		} else {
			monitor.ExtChecks["Process"].message = fmt.Sprintf("PROCESS: Process %s is running, %d threads", monitor.ExtChecks["Process"].Name, i)
			monitor.ExtChecks["Process"].msgStatus = fmt.Sprintf("PROCESS: Process %s is running, %d threads", monitor.ExtChecks["Process"].Name, i)
		}
		monitor.ExtChecks["Process"].Unlock()
	}
}

func (monitor *Monitor) Sync() {
	exit := func() {
		monitor.ExtChecks["Sync"].Lock()
		monitor.ExtChecks["Sync"].status = true
		monitor.ExtChecks["Sync"].message = fmt.Sprintf("SYNC: Can't check sync status")
		monitor.ExtChecks["Sync"].msgStatus = fmt.Sprintf("SYNC: Can't check sync status")
		monitor.ExtChecks["Sync"].Unlock()
	}
	ticker := time.NewTicker(time.Duration(extChecksInterval) * time.Second)
	f := func() {
		select {
		case <-ticker.C:
		}
	}
	for ; true; f() {
		var out bytes.Buffer
		var masterchainblocktime int64
		var unixtime int64
		cmd := exec.Command(monitor.TonPath+"/ton/build/validator-engine-console/validator-engine-console", "-a", "127.0.0.1:3030", "-k", "client", "-p", "server.pub", "-c", "getstats", "-c", "quit")
		cmd.Dir = monitor.KeysPath
		cmd.Stdout = &out
		cmd.Stdin = strings.NewReader("")
		err := cmd.Run()
		if err != nil {
			err = fmt.Errorf("Error running external validator-engine-console: %s", err)
			log.Println(err)
			exit()
			return
		}
		scanner := bufio.NewScanner(&out)
		for scanner.Scan() {
			s := scanner.Text()
			if strings.Contains(s, "masterchainblocktime") {
				words := strings.Fields(s)
				masterchainblocktime, _ = strconv.ParseInt(words[1], 10, 64)
			}
			if strings.Contains(s, "unixtime") {
				words := strings.Fields(s)
				unixtime, _ = strconv.ParseInt(words[1], 10, 64)
			}
		}
		TIME_DIFF := masterchainblocktime - unixtime
		monitor.ExtChecks["Sync"].Lock()
		if TIME_DIFF <= int64(monitor.ExtChecks["Sync"].Threshold) {
			monitor.ExtChecks["Sync"].status = true
		} else {
			monitor.ExtChecks["Sync"].status = false
		}
		//Node is in sync (no problems): status = false
		if monitor.ExtChecks["Sync"].status {
			monitor.ExtChecks["Sync"].message = fmt.Sprintf("SYNC: ALERT: The node is out of sync, TIME_DIFF = %d, thresold %.0f", TIME_DIFF, monitor.ExtChecks["Sync"].Threshold)
		} else {
			monitor.ExtChecks["Sync"].message = fmt.Sprintf("SYNC: The node is in sync finally: TIME_DIFF = %d, thresold %.0f", TIME_DIFF, monitor.ExtChecks["Sync"].Threshold)
		}
		monitor.ExtChecks["Sync"].msgStatus = fmt.Sprintf("SYNC: Sync status: TIME_DIFF = %d", TIME_DIFF)
		monitor.ExtChecks["Sync"].Unlock()
	}
}

func (monitor *Monitor) IsActive() {
	exit := func() {
		monitor.ExtChecks["IsActive"].Lock()
		monitor.ExtChecks["IsActive"].status = true
		monitor.ExtChecks["IsActive"].message = fmt.Sprintf("IS ACTIVE?: Can't check status")
		monitor.ExtChecks["IsActive"].msgStatus = fmt.Sprintf("IS ACTIVE?: Can't check status")
		monitor.ExtChecks["IsActive"].Unlock()
	}
	ticker := time.NewTicker(time.Duration(extChecksInterval) * time.Second)
	f := func() {
		select {
		case <-ticker.C:
		}
	}
	for ; true; f() {
		var out bytes.Buffer
		var isActive = false
		var adnlAddr string
		var adnlCurr string
		var adnlPrev string
		filename := monitor.KeysPath + "/elections/" + monitor.hostname + "-election-adnl-key"
		sFile, err := os.Open(filename)
		if err != nil {
			err = fmt.Errorf("Can't read %s, please check KeysPath", filename)
			log.Println(err)
			exit()
			return
		} else {
			fileScanner := bufio.NewScanner(sFile)
			fileScanner.Split(bufio.ScanLines)
			for fileScanner.Scan() {
				s := fileScanner.Text()
				if strings.Contains(s, "created new key") {
					words := strings.Fields(s)
					adnlAddr = words[3]
					//log.Printf("Current ADNL Addr: %s", adnlAddr)
				}
			}
			sFile.Close()
		}
		currentFile, err := os.Open("current")
		if err != nil {
			log.Println("No current ADNL file, saving...")
			saveADNL(adnlAddr, "current")
			adnlCurr = adnlAddr
		} else {
			fileScanner := bufio.NewScanner(currentFile)
			fileScanner.Split(bufio.ScanLines)
			for fileScanner.Scan() {
				adnlCurr = fileScanner.Text()
			}
			currentFile.Close()
			//log.Printf("Read current ADNL from file: %s\n", adnlCurr)
		}
		if adnlCurr != adnlAddr {
			//ADNL changed
			monitor.ExtChecks["IsActive"].Lock()
			monitor.ExtChecks["IsActive"].adnlChanged = true
			monitor.ExtChecks["IsActive"].Unlock()
			os.Rename("current", "previous")
			saveADNL(adnlAddr, "current")
			adnlPrev = adnlCurr
			adnlCurr = adnlAddr
		}
		previousFile, err := os.Open("previous")
		if err == nil {
			fileScanner := bufio.NewScanner(previousFile)
			fileScanner.Split(bufio.ScanLines)
			for fileScanner.Scan() {
				adnlPrev = fileScanner.Text()
			}
			previousFile.Close()
			//log.Printf("Read previous ADNL from file: %s\n", adnlPrev)
		}

		cmd := exec.Command(monitor.TonPath+"/ton/build/lite-client/lite-client", "-a", "127.0.0.1:3031", "-p", monitor.KeysPath+"/liteserver.pub", "-rc", "getconfig 34")
		cmd.Stdout = &out
		cmd.Stdin = strings.NewReader("")
		err = cmd.Run()
		if err != nil {
			err = fmt.Errorf("Error running external lite-client: %s", err)
			log.Println(err)
			exit()
			return
		}
		scanner := bufio.NewScanner(&out)
		for scanner.Scan() {
			s := scanner.Text()
			if strings.Contains(s, adnlCurr) {
				isActive = true
				monitor.ExtChecks["IsActive"].Lock()
				monitor.ExtChecks["IsActive"].adnlChanged = false
				monitor.ExtChecks["IsActive"].Unlock()
				//words := strings.Fields(s)
				//weightI, _ := strconv.ParseInt(strings.Split(words[2], ":")[1], 10, 64)
				//log.Printf("%d", weightI)
			}
			if adnlPrev != "" && strings.Contains(s, adnlPrev) {
				isActive = true
				//words := strings.Fields(s)
				//weightI, _ := strconv.ParseInt(strings.Split(words[2], ":")[1], 10, 64)
				//log.Printf("%d", weightI)
			}
		}
		monitor.ExtChecks["IsActive"].Lock()
		monitor.ExtChecks["IsActive"].status = !isActive
		//In the active set (no problems): status = false
		if monitor.ExtChecks["IsActive"].status {
			monitor.ExtChecks["IsActive"].message = fmt.Sprintf("IS ACTIVE?: ALERT: Validator is not in the active set (or ADNL has changed recently), ADNL current: %s, ADNL previous: %s", adnlCurr, adnlPrev)
			monitor.ExtChecks["IsActive"].msgStatus = fmt.Sprintf("IS ACTIVE?: Validator is not in the active set, ADNL current: %s, ADNL previous: %s", adnlCurr, adnlPrev)
		} else {
			monitor.ExtChecks["IsActive"].message = fmt.Sprintf("IS ACTIVE?: Validator is in the active set now, ADNL current: %s, ADNL previous: %s", adnlCurr, adnlPrev)
			monitor.ExtChecks["IsActive"].msgStatus = fmt.Sprintf("IS ACTIVE?: Validator is in the active set, ADNL current: %s, ADNL previous: %s", adnlCurr, adnlPrev)
		}
		monitor.ExtChecks["IsActive"].Unlock()
	}
}

func (monitor *Monitor) IsInElections() {
	exit := func() {
		monitor.ExtChecks["IsInElections"].Lock()
		monitor.ExtChecks["IsInElections"].status = true
		monitor.ExtChecks["IsInElections"].message = fmt.Sprintf("IS IN ELECTIONS?: Can't check status")
		monitor.ExtChecks["IsInElections"].msgStatus = fmt.Sprintf("IS IN ELECTIONS?: Can't check status")
		monitor.ExtChecks["IsInElections"].Unlock()
	}
	ticker := time.NewTicker(time.Duration(extChecksInterval) * time.Second)
	f := func() {
		select {
		case <-ticker.C:
		}
	}
	for ; true; f() {
		var out bytes.Buffer
		var isInElections = false
		var pubKey string
		var stake int64
		var status bool
		isNotActive, err := monitor.isElectionsNotActive()
		if err != nil {
			err = fmt.Errorf("Error running external lite-client: %s", err)
			log.Println(err)
			exit()
			return
		}
		if !isNotActive {
			filename := monitor.KeysPath + "/elections/" + monitor.hostname + "-request-dump2"
			//log.Println(filename)
			sFile, err := os.Open(filename)
			if err != nil {
				err = fmt.Errorf("Can't read %s, please check KeysPath", filename)
				log.Println(err)
				exit()
				return
			} else {
				fileScanner := bufio.NewScanner(sFile)
				fileScanner.Split(bufio.ScanLines)
				for fileScanner.Scan() {
					s := fileScanner.Text()
					if strings.Contains(s, "Provided a valid Ed25519 signature") {
						words := strings.Fields(s)
						pubKey = words[10]
						//log.Printf("Current Validator Public Key: %s", pubKey)
					}
				}
				sFile.Close()
			}
			pubKeyBig := new(big.Int)
			pubKeyBig.SetString(pubKey, 16)
			pubKeyBigString := pubKeyBig.Text(10)
			//log.Printf("Current Validator Public Key Big Int: %s", pubKeyBigString)
			cmd := exec.Command(monitor.TonPath+"/ton/build/lite-client/lite-client", "-a", "127.0.0.1:3031", "-p", monitor.KeysPath+"/liteserver.pub", "-rc", "runmethod -1:3333333333333333333333333333333333333333333333333333333333333333 participant_list")
			cmd.Stdout = &out
			cmd.Stdin = strings.NewReader("")
			err = cmd.Run()
			if err != nil {
				err = fmt.Errorf("Error running external lite-client: %s", err)
				log.Println(err)
				exit()
				return
			}
			scanner := bufio.NewScanner(&out)
		Label:
			for scanner.Scan() {
				s := scanner.Text()
				if strings.Contains(s, pubKeyBigString) {
					isInElections = true
					words := strings.SplitAfterN(s, pubKeyBigString, 2)
					//log.Println(words)
					words = strings.Split(words[1], "]")
					//log.Println(words[0])
					words = strings.Fields(words[0])
					//log.Println(words[0])
					stakeNano, _ := strconv.ParseInt(words[0], 10, 64)
					//log.Println(stakeNano)
					stake = stakeNano / 1000000000
					break Label
				}
			}

			//If we have voted, ADNL should have changed, and we should be in the elections
			//If monitor.ExtChecks["IsActive"].adnlChanged is true, and !isInElections: status = true
			monitor.ExtChecks["IsActive"].Lock()
			if monitor.ExtChecks["IsActive"].adnlChanged && !isInElections {
				status = true
			}
			monitor.ExtChecks["IsActive"].Unlock()
		}
		monitor.ExtChecks["IsInElections"].Lock()
		monitor.ExtChecks["IsInElections"].status = status
		if isNotActive {
			monitor.ExtChecks["IsInElections"].message = fmt.Sprintf("IS IN ELECTIONS?: Elections closed")
			monitor.ExtChecks["IsInElections"].msgStatus = fmt.Sprintf("IS IN ELECTIONS?: Elections closed")
		} else if isInElections {
			monitor.ExtChecks["IsInElections"].message = fmt.Sprintf("IS IN ELECTIONS?: Validator is in the elections, stake: %d", stake)
			monitor.ExtChecks["IsInElections"].msgStatus = fmt.Sprintf("IS IN ELECTIONS?: Validator is in the elections, stake: %d", stake)
		} else if monitor.ExtChecks["IsInElections"].status {
			monitor.ExtChecks["IsInElections"].message = fmt.Sprintf("IS IN ELECTIONS?: ALERT: Validator is not in the elections")
			monitor.ExtChecks["IsInElections"].msgStatus = fmt.Sprintf("IS IN ELECTIONS?: Validator is not in the elections")
		}
		monitor.ExtChecks["IsInElections"].Unlock()
	}
}

func (monitor *Monitor) IsNext() {
	exit := func() {
		monitor.ExtChecks["IsNext"].Lock()
		monitor.ExtChecks["IsNext"].status = true
		monitor.ExtChecks["IsNext"].message = fmt.Sprintf("IS NEXT?: Can't check status")
		monitor.ExtChecks["IsNext"].msgStatus = fmt.Sprintf("IS NEXT?: Can't check status")
		monitor.ExtChecks["IsNext"].Unlock()
	}
	ticker := time.NewTicker(time.Duration(extChecksInterval) * time.Second)
	f := func() {
		select {
		case <-ticker.C:
		}
	}
	for ; true; f() {
		var out bytes.Buffer
		var isActive = false
		var isEmpty = false
		var adnlAddr string
		filename := monitor.KeysPath + "/elections/" + monitor.hostname + "-election-adnl-key"
		//log.Println(filename)
		sFile, err := os.Open(filename)
		if err != nil {
			err = fmt.Errorf("Can't read %s, please check KeysPath", filename)
			log.Println(err)
			exit()
			return
		} else {
			fileScanner := bufio.NewScanner(sFile)
			fileScanner.Split(bufio.ScanLines)
			for fileScanner.Scan() {
				s := fileScanner.Text()
				if strings.Contains(s, "created new key") {
					words := strings.Fields(s)
					adnlAddr = words[3]
					//log.Printf("Next ADNL Addr: %s", adnlAddr)
				}
			}
			sFile.Close()
		}

		cmd := exec.Command(monitor.TonPath+"/ton/build/lite-client/lite-client", "-a", "127.0.0.1:3031", "-p", monitor.KeysPath+"/liteserver.pub", "-rc", "getconfig 36")
		cmd.Stdout = &out
		cmd.Stdin = strings.NewReader("")
		err = cmd.Run()
		if err != nil {
			err = fmt.Errorf("Error running external lite-client: %s", err)
			log.Println(err)
			exit()
			return
		}
		scanner := bufio.NewScanner(&out)
		for scanner.Scan() {
			s := scanner.Text()
			if strings.Contains(s, adnlAddr) {
				isActive = true
				//words := strings.Fields(s)
				//weightI, _ := strconv.ParseInt(strings.Split(words[2], ":")[1], 10, 64)
				//log.Printf("%d", weightI)
			}
			if strings.Contains(s, "ConfigParam(36) = (null)") {
				isEmpty = true
			}
		}
		monitor.ExtChecks["IsNext"].Lock()
		monitor.ExtChecks["IsNext"].status = false
		//Next set is not empty and not active (not in the next set): status = true
		if !isActive && !isEmpty {
			monitor.ExtChecks["IsNext"].status = true
		}
		if monitor.ExtChecks["IsNext"].status {
			monitor.ExtChecks["IsNext"].message = fmt.Sprintf("IS NEXT?: ALERT: Validator is not in the next set, ADNL address: %s", adnlAddr)
			monitor.ExtChecks["IsNext"].msgStatus = fmt.Sprintf("IS NEXT?: Validator is not in the next set, ADNL address: %s", adnlAddr)
		} else if isActive {
			monitor.ExtChecks["IsNext"].message = fmt.Sprintf("IS NEXT?: Validator is in the next set, ADNL address: %s", adnlAddr)
			monitor.ExtChecks["IsNext"].msgStatus = monitor.ExtChecks["IsNext"].message
		} else if isEmpty {
			monitor.ExtChecks["IsNext"].message = fmt.Sprintf("IS NEXT?: The next set is empty")
			monitor.ExtChecks["IsNext"].msgStatus = monitor.ExtChecks["IsNext"].message
		}
		monitor.ExtChecks["IsNext"].Unlock()
	}
}

//helper functions
func (monitor *Monitor) isElectionsNotActive() (isNotActive bool, err error) {
	var out bytes.Buffer
	cmd := exec.Command(monitor.TonPath+"/ton/build/lite-client/lite-client", "-a", "127.0.0.1:3031", "-p", monitor.KeysPath+"/liteserver.pub", "-rc", "runmethod -1:3333333333333333333333333333333333333333333333333333333333333333 active_election_id")
	cmd.Stdout = &out
	cmd.Stdin = strings.NewReader("")
	err = cmd.Run()
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		s := scanner.Text()
		if strings.Contains(s, "result:  [ 0 ]") {
			isNotActive = true
			return
		}
	}
	return
}

func saveADNL(adnl string, file string) {
	f, _ := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	datawriter := bufio.NewWriter(f)
	_, _ = datawriter.WriteString(adnl + "\n")
	datawriter.Flush()
	f.Close()
	return
}
