# ftvmon

Free TON Validator’s Node Monitoring and Alerting. Uses telegram as an endpoint for status messages and alerts, supports multiple users. Sends alerts or reports current status for all metrics enabled if the user issues the `/status` command.

## Installation

Tested on Ubuntu 18.04. 
Install Go first:
```bash
cd ~
curl -O https://dl.google.com/go/go1.14.3.linux-amd64.tar.gz
sudo tar -xvf go1.14.3.linux-amd64.tar.gz -C /usr/local
sudo chown -R root:root /usr/local/go
```
Create Go Workspace:
```bash
mkdir -p $HOME/go/{bin,src}
```
Add the following lines to `~/.profile`:
```bash
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin:/usr/local/go/bin
```
Update your shell:
```bash
. ~/.profile
```
Instal dependencies and build:
```bash
go get -u gopkg.in/tucnak/telebot.v2
go get -u github.com/shirou/gopsutil
go get -u golang.org/x/sys/unix
go get -u github.com/4hash/ftvmon
cd ~/go/src/github.com/4hash/ftvmon
go build
```
After building, you may copy the resulting executable and conf.json to another folder:
```bash
mkdir ~/ftvmon
cp ftvmon ~/ftvmon
cp conf.json ~/ftvmon
cd ~/ftvmon
```

## Usage
After editing the conf.json and running the executable for the first time, issue the `/subscribe` command to the bot, to subscribe to alerts (one time only, subscribed IDs are stored in "*subscribers*" file). Without the `/subscribe` command you will not receive any alerts, but will be able to get current status using `/status` command anytime (if authorized).
Edit the *conf.json* file.
Insert your bot's telegram token, created using BotFather:
```json
"Token":"1122334455:AABBCCDDEEFFaaaaaaaaaaaaaaaaaaaabbb",
```
Add telegram usernames of users that will be authorized to `/subscribe` to alerts and get `/status` updates:
```json
   "Authorized":[
      "UserA",
      "UserB"
   ],
```
Add paths to keys and FreeTON C++ Validator's Node installation:

```json
   "TonPath":"/home/freeton/net.ton.dev",
   "KeysPath":"/home/freeton/ton-keys",
```
In the following system performance metrics `"Checks"` section, edit the thresholds that will trigger alerts and specific `"Check"`'s parameters. Sends a message when a condition arises (above threshold) and when it clears (below threshold). Every check (metric) can be disabled. `"Checks"` are running continiously.
Name of the proccess to monitor (an alert will be sent if the proccess is not found):
```json
   "Checks":{
      "Process":{
         "Enabled":true,
         "Name":"validator-engine"
      },

```
CPU Load percentage (measured on 5s intervals):
```json
      "CPU":{
         "Enabled":true,
         "Threshold":90.0
      },
```
Memory used, %:
```json
      "Mem":{
         "Enabled":true,
         "Threshold":90.0
      },
```
Percentage of used disk space on the volume, corresponding to `"Path"`:
```json
      "DiskSpace":{
         "Enabled":true,
         "Path":"/var/ton-work",
         "Threshold":80.0
      },
```
Aggregate disk IOPS (reads + writes), do not prefix sda with /dev/:
```json
      "DiskIOPS":{
         "Enabled":true,
         "dev":"sda",
         "Threshold":4000.0
      },
```
Disk I/O % utilization (derived from Weighted time spent doing I/Os) is the most meaningful disk counter, device saturation occurs when this value is close to 100% for a single disk (for RAIDs capable of multiple I/O operations simultaneously it can be higher):
```json
      "DiskIOUtil":{
         "Enabled":true,
         "dev":"sda",
         "Threshold":80.0
      },
```
Aggregate disk Megabytes per second:
```json
      "DiskMBps":{
         "Enabled":true,
         "dev":"sda",
         "Threshold":300.0
      },
```
Aggregate (all interfaces) network Megabytes per second:
```json
      "NetMbs":{
         "Enabled":true,
         "Threshold":100.0
      }
```
The following `"ExtChecks"` are run every minute and invoke external processes.
Sync checks sync status (TIME_DIFF) of the node:
```json
      "Sync":{
         "Enabled":true,
         "Threshold":-30
      },
```
Is validator’s node in the active set? Checks status using ADNL address, since default scripts overwrite ADNL key file after submitting a stake for the elections, software saves previous ADNL address. Sends an alert if neither of the ADNL keys can be found in the active set:
```json
      "IsActive":{
         "Enabled":true
      },
```
Is validator’s node in the elections? During elections, if the validator tried to submit a stake for the elections, but its public key can’t be found in the list of election participants, sends an alert. If the validator is found, adds stake amount to status message:
```json
      "IsInElections":{
         "Enabled":true
      },
```
Is validator’s node in the next set? If the next set is active, checks status using current ADNL key and sends an alert if the validator is not found.
```json
      "IsNext":{
         "Enabled":true
      }
```
In the following `"Logfile"` section, monitoring of log events is configured. **ftvmon** can monitor multiple logs simultaneously in real-time, with multiple event-matching criteria per log. Event-matching can be done against simple substring (`"IsRegex":false`) or using regex (`"IsRegex":true`). If you use regex, double backslashes "\\" are required to put literal "\" characters in the regex string (json files limitation). Log files are seeked to the end at launch. An alert message (`"MessageOn"`) for every event class can be triggered by a single event every time (if `"Window"` parameter is set to 0) or by a number of events exceeding a predefined threshold during a predefined time window (`"Window"`, minutes), in this case the system will send an off message (`"MessageOff"`) if the condition clears (i.e. if the number of events during last n minutes becomes lower than a threshold set in the config). `"IncludeRaw"` parameter controls, if the `"MessageOn"` alert will be suffixed with the original log record that triggered the alert (with `"Window"` this will be the last log record that increased the number of events up to the `"Threshold"` within last `"Window"`: n minutes):
```json
   "Logfiles":[
      {
         "Enabled":true,
         "File":"/var/ton-work/node.log",
         "Events":[
            {
               "Enabled":false,
               "Match":"SLOW",
               "IsRegex":false,
               "MessageOn":"Too many SLOW records in the log, more than 150 within last 1 minute",
               "MessageOff":"The amount of SLOW records is back to normal, less than 150 within last 1 minute",
               "Threshold":150,
               "Window":1,
               "IncludeRaw":true
            },
            {
               "Enabled":true,
               "Match":"PosixError : Connection refused",
               "IsRegex":false,
               "MessageOn":"Connection issues!!!",
               "MessageOff":"",
               "Threshold":1,
               "Window":0,
               "IncludeRaw":true
            }
         ]
      },
      {
         "Enabled":true,
         "File":"/home/freeton/net.ton.dev/scripts/validator.log",
         "Events":[
            {
               "Enabled":true,
               "Match":"prepared for elections",
               "IsRegex":false,
               "MessageOn":"Prepared for elections",
               "MessageOff":"If Window is 0, MessageOn will be triggered every time the event occurs (no MessageOff) ",
               "Threshold":1,
               "Window":0,
               "IncludeRaw":true
            },
            {
               "Enabled":true,
               "Match":"submitTransaction attempt.*FAIL",
               "IsRegex":true,
               "MessageOn":"Failed to send or recover stake",
               "MessageOff":"Double backslashes are required to put literal \ characters in the regex string",
               "Threshold":1,
               "Window":0,
               "IncludeRaw":true
            }
         ]
      }
   ]
}
```
Run **ftvmon**. 

## Adding metrics
Uses run-time reflection, a metric can be added by adding a function (returning status and setting corresponding messages) to checks.go, and creating a config entry with the name of the function.

## TODO
* Logging levels
* Add weight to validator's ective set and next set checks
* Multiple validator's servers support (with agents)
* Zabbix integration
* Docker counters support
* Native calls to services (in place of invoking external validator-engine-console and lite-client)
