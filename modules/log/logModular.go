package log

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/natefinch/lumberjack"
	"github.com/op/go-logging"
)

// DefaultLogPath ...
const DefaultLogPath = "/data/log/kumi/kumi.log"

// Logger : logger struct
type Logger struct {
	module  string
	logger  *logging.Logger
	format  logging.Formatter
	logpath string
	f       *os.File
}

// NewLogger : Constructor of go-logging
func NewLogger(module string, args ...string) *Logger {
	var logPath string

	if len(args) > 0 {
		logPath = args[0]
	} else {
		logPath = DefaultLogPath
	}

	file, _ := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0664)

	tmp := &Logger{
		module: module,
		logger: logging.MustGetLogger(module),
		format: logging.MustStringFormatter(
			// `%{color}%{time:2006-01-02 15:04:05.000} %{level:.4s} ▶ [` + module + `] %{color:reset}%{message}`,
			`%{time:2006-01-02 15:04:05.000} %{level:.4s} ▶ [` + module + `] %{message}`,
		),
		logpath: logPath,
		f:       file,
	}

	setOutPut(logPath, tmp)
	return tmp
}

func setOutPut(_logPath string, l *Logger) *Logger {
	backend1 := logging.NewLogBackend(os.Stderr, "", 0)
	backend2 := logging.NewLogBackend(&lumberjack.Logger{
		Filename: _logPath,
	}, "", 0)

	backend2Formatter := logging.NewBackendFormatter(backend2, l.format)
	backend1Leveled := logging.AddModuleLevel(backend1)
	backend1Leveled.SetLevel(logging.ERROR, "")

	l.logger.SetBackend(logging.MultiLogger(backend1Leveled, backend2Formatter))

	return l
}

// GetLogger : Caller's Logger Object Return
func (l *Logger) GetLogger() *Logger {
	return l
}

// GetModule : Caller's Logger Module Name Return
func (l *Logger) GetModule() string {
	return l.module
}

func mergeMsg(msg []interface{}) string {
	var msgArr []string
	for _, v := range msg {
		switch v.(type) {
		case int:
			msgArr = append(msgArr, strconv.Itoa(v.(int)))
		case float64:
			msgArr = append(msgArr, strconv.FormatFloat(v.(float64), 'E', -1, 64))
		case string:
			msgArr = append(msgArr, v.(string))
		default:
			msgArr = append(msgArr, fmt.Sprintf("%+v", v))
		}
	}

	return strings.Join(msgArr, "")
}

// Info : Info level Interface
func (l *Logger) Info(msg ...interface{}) {
	l.checkLogFileExists()
	l.logger.Info(mergeMsg(msg))
}

// Debug :  Debug level Interface
func (l *Logger) Debug(msg ...interface{}) {
	l.checkLogFileExists()
	l.logger.Debug(mergeMsg(msg))
}

// Warn : Warning level Interface
func (l *Logger) Warn(msg ...interface{}) {
	l.checkLogFileExists()
	l.logger.Warning(mergeMsg(msg))
}

// Error : Error level Interface
func (l *Logger) Error(msg ...interface{}) {
	l.checkLogFileExists()
	l.logger.Error(mergeMsg(msg))
}

// Fatal : Fatal level Interface
func (l *Logger) Fatal(msg ...interface{}) {
	l.checkLogFileExists()
	l.logger.Fatal(mergeMsg(msg))
}

// Panic : Panic level Interface
func (l *Logger) Panic(msg ...interface{}) {
	l.checkLogFileExists()
	l.logger.Panic(mergeMsg(msg))
}

func (l *Logger) checkLogFileExists() {
	currLogger := l.GetLogger()

	if _, err := os.Stat(currLogger.logpath); os.IsNotExist(err) {
		fmt.Println("Existing logPath doesn't Exist")
		l.f.Close()
		if l.f, err = os.OpenFile(l.f.Name(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0664); err != nil {
			return
		}

		setOutPut(l.f.Name(), l)
	}
}
