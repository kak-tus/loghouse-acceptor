package listener

var levels = map[int]string{
	0: "FATAL",
	1: "FATAL",
	2: "FATAL",
	3: "ERROR",
	4: "WARN",
	5: "INFO",
	6: "INFO",
	7: "DEBUG",
}

var facilities = map[int]string{
	0:  "kern",
	1:  "user",
	2:  "mail",
	3:  "daemon",
	4:  "auth",
	5:  "syslog",
	6:  "lpr",
	7:  "news",
	8:  "uucp",
	9:  "cron",
	10: "security",
	11: "ftp",
	12: "ntp",
	13: "logaudit",
	14: "logalert",
	15: "clock",
	16: "local0",
	17: "local1",
	18: "local2",
	19: "local3",
	20: "local4",
	21: "local5",
	22: "local6",
	23: "local7",
}
