package main

import (
	"fmt"
	rotatelogs "github.com/lestrrat/go-file-rotatelogs"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"
)

func panicIfError(err error, message string) {
	if err != nil {
		fmt.Println(message)
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func initLogger(logdir, logLevel string, doStd bool) {

	var writer io.Writer
	var writers []io.Writer

	if logdir != "" {
		folderPath, err := filepath.Abs(logdir)
		panicIfError(err, fmt.Sprintf("Error on parsing log path: %s", logdir))

		abspath, err := filepath.Abs(path.Join(logdir, "run.log"))
		panicIfError(err, fmt.Sprintf("Error on parsing log file path: %s", logdir))

		err = os.MkdirAll(folderPath, os.ModePerm)
		panicIfError(err, fmt.Sprintf("Error on creating log dir: %s", folderPath))

		fmt.Println("Will be logged to ", abspath)
		writers = append(writers, RotateLog(abspath))
	}
	if doStd {
		// stdout only
		fmt.Println("Will be logged to stdout")

		writers = append(writers, os.Stdout)
	}

	writer = io.MultiWriter(writers...)
	logrus.SetOutput(writer)

	// Only log the warning severity or above.
	switch logLevel {
	case "panic":
		logrus.SetLevel(logrus.PanicLevel)
	case "fatal":
		logrus.SetLevel(logrus.FatalLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "trace":
		logrus.SetLevel(logrus.TraceLevel)
	default:
		fmt.Println("Unknown level", logLevel, "Set to INFO")
		logrus.SetLevel(logrus.InfoLevel)
	}

	Formatter := new(logrus.TextFormatter)
	Formatter.ForceColors = false
	Formatter.DisableColors = true
	Formatter.TimestampFormat = "06-01-02 15:04:05.000000"
	Formatter.FullTimestamp = true
	logrus.SetFormatter(Formatter)

	// redirect standard log to logrus
	logrus.SetReportCaller(true)
	panicLog, _ := filepath.Abs(path.Join(logdir, "panic.log"))
	fatalLog, _ := filepath.Abs(path.Join(logdir, "fatal.log"))
	warnLog, _ := filepath.Abs(path.Join(logdir, "warn.log"))
	errorLog, _ := filepath.Abs(path.Join(logdir, "error.log"))
	infoLog, _ := filepath.Abs(path.Join(logdir, "info.log"))
	debugLog, _ := filepath.Abs(path.Join(logdir, "debug.log"))
	traceLog, _ := filepath.Abs(path.Join(logdir, "trace.log"))
	writerMap := lfshook.WriterMap{
		logrus.PanicLevel: RotateLog(panicLog),
		logrus.FatalLevel: RotateLog(fatalLog),
		logrus.WarnLevel:  RotateLog(warnLog),
		logrus.ErrorLevel: RotateLog(errorLog),
		logrus.InfoLevel:  RotateLog(infoLog),
		logrus.DebugLevel: RotateLog(debugLog),
		logrus.TraceLevel: RotateLog(traceLog),
	}
	logrus.AddHook(lfshook.NewHook(
		writerMap,
		Formatter,
	))

}

func RotateLog(abspath string) *rotatelogs.RotateLogs {
	logFile, err := rotatelogs.New(
		abspath+"%Y%data%d%H%M.log",
		rotatelogs.WithLinkName(abspath+".log"),
		rotatelogs.WithMaxAge(24*time.Hour*7),
		rotatelogs.WithRotationTime(time.Hour*24),
	)
	panicIfError(err, "err init log")
	return logFile
}
