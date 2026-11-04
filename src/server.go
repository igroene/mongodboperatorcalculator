package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	MO "github.com/igroene/mongodboperatorcalculator/src/mongodboperatorcalculator"
)

const maxRequestBodyBytes = 1 << 20

func main() {
	address := flag.String("address", "0.0.0.0", "IP address or hostname to bind to")
	port := flag.Int("port", 8080, "TCP port to listen on")
	logLevel := flag.String("loglevel", "INFO", "log level: ERROR, INFO, or DEBUG")
	showVersion := flag.Bool("version", false, "print the calculator version and exit")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "MongoDB Operator Calculator %s\n\n", MO.VERSION)
		fmt.Fprintln(flag.CommandLine.Output(), "Endpoints:")
		fmt.Fprintln(flag.CommandLine.Output(), "  GET  /supported   Return supported layouts")
		fmt.Fprintln(flag.CommandLine.Output(), "  POST /calculator  Calculate a MongoDB configuration")
		fmt.Fprintln(flag.CommandLine.Output(), "  GET  /healthz     Liveness and readiness check")
		fmt.Fprintln(flag.CommandLine.Output(), "\nFlags:")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showVersion {
		fmt.Println(MO.VERSION)
		return
	}
	logger := newLogger(*logLevel)
	server := &http.Server{
		Addr:              *address + ":" + strconv.Itoa(*port),
		Handler:           routes(logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger.Infof("starting MongoDB Operator Calculator %s on %s", MO.VERSION, server.Addr)
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		logger.Errorf("listen failed: %v", err)
		os.Exit(1)
	}
	if err := serve(ctx, server, listener, logger, 10*time.Second); err != nil {
		logger.Errorf("server stopped: %v", err)
		os.Exit(1)
	}
}

// serve runs the server until ctx is cancelled, then waits for in-flight
// requests to finish. Serve returns as soon as Shutdown starts, so serve
// must not return until Shutdown itself has returned.
func serve(ctx context.Context, server *http.Server, listener net.Listener, logger *logger, drainTimeout time.Duration) error {
	shutdownDone := make(chan error, 1)
	go func() {
		<-ctx.Done()
		logger.Infof("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), drainTimeout)
		defer cancel()
		shutdownDone <- server.Shutdown(shutdownCtx)
	}()
	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	if err := <-shutdownDone; err != nil {
		return fmt.Errorf("shutdown failed: %w", err)
	}
	logger.Infof("shutdown complete")
	return nil
}

const (
	levelError = iota
	levelInfo
	levelDebug
)

type logger struct {
	level int
	out   *log.Logger
}

func newLogger(level string) *logger {
	levels := map[string]int{"ERROR": levelError, "INFO": levelInfo, "DEBUG": levelDebug}
	l, ok := levels[strings.ToUpper(level)]
	if !ok {
		l = levelInfo
	}
	return &logger{level: l, out: log.New(os.Stderr, "mongodboperatorcalculator ", log.LstdFlags)}
}

func (l *logger) logf(level int, name, format string, args ...any) {
	if level <= l.level {
		l.out.Printf(name+": "+format, args...)
	}
}
func (l *logger) Errorf(format string, args ...any) { l.logf(levelError, "ERROR", format, args...) }
func (l *logger) Infof(format string, args ...any)  { l.logf(levelInfo, "INFO", format, args...) }
func (l *logger) Debugf(format string, args ...any) { l.logf(levelDebug, "DEBUG", format, args...) }

func routes(logger *logger) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/supported", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		var conf MO.Configuration
		conf.Init()
		writeJSON(w, http.StatusOK, conf)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": MO.VERSION})
	})
	mux.HandleFunc("/calculator", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		calculateRequest(w, r, logger)
	})
	return mux
}

func calculateRequest(w http.ResponseWriter, r *http.Request, logger *logger) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("request body exceeds %d bytes", maxRequestBodyBytes))
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to read request body: %v", err))
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		writeError(w, http.StatusBadRequest, "empty request body")
		return
	}
	var request MO.ConfigurationRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "malformed JSON: "+err.Error())
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "malformed JSON: unexpected data after the request object")
		return
	}
	var conf MO.Configuration
	conf.Init()
	var calculator MO.MongoDBOperatorCalculator
	request = calculator.Init(request, conf)
	calculationErr, message, families := calculator.GetCalculate()
	status := http.StatusOK
	if message.MType == MO.ErrorexecI {
		status = http.StatusBadRequest
	} else if message.MType == MO.OverutilizingI {
		status = http.StatusUnprocessableEntity
	}
	if calculationErr != nil && message.MType != MO.ErrorexecI && message.MType != MO.OverutilizingI {
		logger.Errorf("calculation failed: %v", calculationErr)
	}
	logger.Debugf("calculation returned type %d: %s", message.MType, message.MText)
	if request.Output == MO.ResultOutputFormatHuman {
		output, outputErr := calculator.GetHumanOutput(message, request, families)
		if outputErr != nil {
			logger.Errorf("human output failed: %v", outputErr)
			writeError(w, http.StatusInternalServerError, outputErr.Error())
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write(output.Bytes())
		return
	}
	output, outputErr := calculator.GetJSONOutput(message, request, families)
	if outputErr != nil {
		logger.Errorf("JSON output failed: %v", outputErr)
		writeError(w, http.StatusInternalServerError, outputErr.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(output.Bytes())
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeError(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": text})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
