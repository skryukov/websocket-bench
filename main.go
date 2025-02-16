package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/anycable/websocket-bench/benchmark"
	"golang.org/x/net/websocket"

	"github.com/spf13/cobra"
)

var options struct {
	websocketOrigin    string
	serverType         string
	concurrent         int
	concurrentConnect  int
	sampleSize         int
	initialClients     int
	stepSize           int
	limitPercentile    int
	limitRTT           time.Duration
	payloadPaddingSize int
	localAddrs         []string
	workerListenAddr   string
	workerListenPort   int
	workerAddrs        []string
	totalSteps         int
	interactive        bool
	stepsDelay         int
	commandDelay       float64
	commandDelayChance int
	channel            string
	format             string
	filename           string
}

var (
	version string
	commit  string
)

func init() {
	if version == "" {
		version = "0.2.1"
	}

	if commit != "" {
		version = version + "-" + commit
	}
}

func main() {
	rootCmd := &cobra.Command{Use: "websocket-bench", Short: fmt.Sprintf("websocket benchmark tool (%s)", version)}

	cmdEcho := &cobra.Command{
		Use:   "echo URL",
		Short: "Echo stress test",
		Long:  "Stress test 1 to 1 performance with an echo test",
		Run:   Stress,
	}
	cmdEcho.PersistentFlags().StringVarP(&options.websocketOrigin, "origin", "o", "http://localhost", "websocket origin")
	cmdEcho.PersistentFlags().StringSliceVarP(&options.localAddrs, "local-addr", "l", []string{}, "local IP address to connect from")
	cmdEcho.PersistentFlags().StringVarP(&options.serverType, "server-type", "", "json", "server type to connect to (json, binary, actioncable, phoenix)")
	cmdEcho.PersistentFlags().StringSliceVarP(&options.workerAddrs, "worker-addr", "w", []string{}, "worker address to distribute connections to")
	cmdEcho.Flags().IntVarP(&options.concurrent, "concurrent", "c", 50, "concurrent echo requests")
	cmdEcho.Flags().IntVarP(&options.sampleSize, "sample-size", "s", 10000, "number of echoes in a sample")
	cmdEcho.Flags().IntVarP(&options.stepSize, "step-size", "", 5000, "number of clients to increase each step")
	cmdEcho.Flags().IntVarP(&options.limitPercentile, "limit-percentile", "", 95, "round-trip time percentile to for limit")
	cmdEcho.Flags().IntVarP(&options.payloadPaddingSize, "payload-padding", "", 0, "payload padding size")
	cmdEcho.Flags().DurationVarP(&options.limitRTT, "limit-rtt", "", time.Millisecond*500, "Max RTT at limit percentile")
	cmdEcho.Flags().IntVarP(&options.totalSteps, "total-steps", "", 0, "Run benchmark for specified number of steps")
	cmdEcho.Flags().BoolVarP(&options.interactive, "interactive", "i", false, "Interactive mode (requires user input to move to the next step")
	cmdEcho.Flags().IntVarP(&options.stepsDelay, "steps-delay", "", 0, "Sleep for seconds between steps")
	cmdEcho.Flags().Float64VarP(&options.commandDelay, "command-delay", "", 0, "Sleep for seconds before sending client command")
	cmdEcho.Flags().IntVarP(&options.commandDelayChance, "command-delay-chance", "", 100, "The percentage of commands to add delay to")
	cmdEcho.Flags().StringVarP(&options.format, "format", "f", "", "output format")
	cmdEcho.Flags().StringVarP(&options.filename, "filename", "n", "", "output filename")
	cmdEcho.PersistentFlags().StringVarP(&options.channel, "channel", "", "{\"channel\":\"BenchmarkChannel\"}", "Action Cable channel identifier")
	rootCmd.AddCommand(cmdEcho)

	cmdBroadcast := &cobra.Command{
		Use:   "broadcast URL",
		Short: "Broadcast stress test",
		Long:  "Stress test 1 to many performance with an broadcast test",
		Run:   Stress,
	}
	cmdBroadcast.PersistentFlags().StringVarP(&options.websocketOrigin, "origin", "o", "http://localhost", "websocket origin")
	cmdBroadcast.PersistentFlags().StringSliceVarP(&options.localAddrs, "local-addr", "l", []string{}, "local IP address to connect from")
	cmdBroadcast.PersistentFlags().StringSliceVarP(&options.workerAddrs, "worker-addr", "w", []string{}, "worker address to distribute connections to")
	cmdBroadcast.PersistentFlags().StringVarP(&options.serverType, "server-type", "", "json", "server type to connect to (json, binary, actioncable, phoenix)")
	cmdBroadcast.Flags().IntVarP(&options.concurrent, "concurrent", "c", 4, "concurrent broadcast requests")
	cmdBroadcast.Flags().IntVarP(&options.concurrentConnect, "connect-concurrent", "", 100, "concurrent connection initialization requests")
	cmdBroadcast.Flags().IntVarP(&options.sampleSize, "sample-size", "s", 20, "number of broadcasts in a sample")
	cmdBroadcast.Flags().IntVarP(&options.initialClients, "initial-clients", "", 0, "initial number of clients")
	cmdBroadcast.Flags().IntVarP(&options.stepSize, "step-size", "", 5000, "number of clients to increase each step")
	cmdBroadcast.Flags().IntVarP(&options.limitPercentile, "limit-percentile", "", 95, "round-trip time percentile to for limit")
	cmdBroadcast.Flags().IntVarP(&options.payloadPaddingSize, "payload-padding", "", 0, "payload padding size")
	cmdBroadcast.Flags().DurationVarP(&options.limitRTT, "limit-rtt", "", time.Millisecond*500, "Max RTT at limit percentile")
	cmdBroadcast.Flags().IntVarP(&options.totalSteps, "total-steps", "", 0, "Run benchmark for specified number of steps")
	cmdBroadcast.Flags().BoolVarP(&options.interactive, "interactive", "i", false, "Interactive mode (requires user input to move to the next step")
	cmdBroadcast.Flags().IntVarP(&options.stepsDelay, "steps-delay", "", 0, "Sleep for seconds between steps")
	cmdBroadcast.Flags().Float64VarP(&options.commandDelay, "command-delay", "", 0, "Sleep for seconds before sending client command")
	cmdBroadcast.Flags().IntVarP(&options.commandDelayChance, "command-delay-chance", "", 100, "The percentage of commands to add delay to")
	cmdBroadcast.Flags().StringVarP(&options.format, "format", "f", "", "output format")
	cmdBroadcast.Flags().StringVarP(&options.filename, "filename", "n", "", "output filename")
	cmdBroadcast.PersistentFlags().StringVarP(&options.channel, "channel", "", "{\"channel\":\"BenchmarkChannel\"}", "Action Cable channel identifier")
	rootCmd.AddCommand(cmdBroadcast)

	cmdWorker := &cobra.Command{
		Use:   "worker",
		Short: "Run in worker mode",
		Long:  "Listen for commands",
		Run:   Work,
	}
	cmdWorker.Flags().StringVarP(&options.workerListenAddr, "address", "a", "0.0.0.0", "address to listen on")
	cmdWorker.Flags().IntVarP(&options.workerListenPort, "port", "p", 3000, "port to listen on")
	rootCmd.AddCommand(cmdWorker)

	cmdConnect := &cobra.Command{
		Use:   "connect URL",
		Short: "Connection initialization stress test",
		Long:  "Stress test connection initialzation",
		Run:   Stress,
	}
	cmdConnect.PersistentFlags().StringVarP(&options.websocketOrigin, "origin", "o", "http://localhost", "websocket origin")
	cmdConnect.PersistentFlags().StringSliceVarP(&options.localAddrs, "local-addr", "l", []string{}, "local IP address to connect from")
	cmdConnect.PersistentFlags().StringVarP(&options.serverType, "server-type", "", "json", "server type to connect to (json, binary, actioncable, phoenix)")
	cmdConnect.PersistentFlags().StringSliceVarP(&options.workerAddrs, "worker-addr", "w", []string{}, "worker address to distribute connections to")
	cmdConnect.Flags().IntVarP(&options.concurrent, "concurrent", "c", 50, "concurrent connection requests")
	cmdConnect.Flags().IntVarP(&options.stepSize, "step-size", "", 5000, "number of clients to connect at each step")
	cmdConnect.Flags().IntVarP(&options.totalSteps, "total-steps", "", 0, "Run benchmark for specified number of steps")
	cmdConnect.Flags().BoolVarP(&options.interactive, "interactive", "i", false, "Interactive mode (requires user input to move to the next step")
	cmdConnect.Flags().IntVarP(&options.stepsDelay, "steps-delay", "", 0, "Sleep for seconds between steps")
	cmdConnect.Flags().Float64VarP(&options.commandDelay, "command-delay", "", 0, "Sleep for seconds before sending client command")
	cmdConnect.Flags().IntVarP(&options.commandDelayChance, "command-delay-chance", "", 100, "The percentage of commands to add delay to")
	cmdConnect.Flags().StringVarP(&options.format, "format", "f", "", "output format")
	cmdConnect.Flags().StringVarP(&options.filename, "filename", "n", "", "output filename")
	cmdConnect.PersistentFlags().StringVarP(&options.channel, "channel", "", "{\"channel\":\"BenchmarkChannel\"}", "Action Cable channel identifier")
	rootCmd.AddCommand(cmdConnect)

	rootCmd.Execute()
}

func Stress(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		cmd.Help()
		os.Exit(1)
	}

	config := &benchmark.Config{}
	config.WebsocketURL = args[0]
	config.WebsocketOrigin = options.websocketOrigin
	config.ServerType = options.serverType
	switch cmd.Name() {
	case "echo":
		config.ClientCmd = benchmark.ClientEchoCmd
	case "broadcast":
		config.ClientCmd = benchmark.ClientBroadcastCmd
	case "connect":
	default:
		panic("invalid command name")
	}
	config.PayloadPaddingSize = options.payloadPaddingSize
	config.StepSize = options.stepSize
	config.Concurrent = options.concurrent
	config.ConcurrentConnect = options.concurrentConnect
	config.SampleSize = options.sampleSize
	config.InitialClients = options.initialClients
	config.LimitPercentile = options.limitPercentile
	config.LimitRTT = options.limitRTT
	config.TotalSteps = options.totalSteps
	config.Interactive = options.interactive
	config.StepDelay = time.Duration(options.stepsDelay) * time.Second
	config.CommandDelay = time.Duration(options.commandDelay) * time.Second
	config.CommandDelayChance = options.commandDelayChance

	var writer io.Writer
	if options.filename == "" {
		writer = os.Stdout
	} else {
		var cancel context.CancelFunc
		writer, cancel = openFileWriter()
		defer cancel()
	}

	if options.format == "json" {
		config.ResultRecorder = benchmark.NewJSONResultRecorder(writer)
	} else {
		config.ResultRecorder = benchmark.NewTextResultRecorder(writer)
	}

	benchmark.CableConfig.Channel = options.channel

	wsconfig, err := websocket.NewConfig(config.WebsocketURL, config.WebsocketOrigin)
	if err != nil {
		panic(fmt.Errorf("failed to generate WS config: %v", err))
	}

	benchmark.RemoteAddr.Config = wsconfig
	benchmark.RemoteAddr.Secure = wsconfig.Location.Scheme == "wss"

	if raddr, host, err := parseRemoteAddr(wsconfig.Location.Host); err != nil {
		panic(fmt.Errorf("failed to parse remote address: %v", err))
	} else {
		benchmark.RemoteAddr.Addr = raddr
		benchmark.RemoteAddr.Host = host
	}

	localAddrs := parseTCPAddrs(options.localAddrs)
	for _, a := range localAddrs {
		config.ClientPools = append(config.ClientPools, benchmark.NewLocalClientPool(a))
	}

	for _, a := range options.workerAddrs {
		rcp, err := benchmark.NewRemoteClientPool(a)
		if err != nil {
			log.Fatal(err)
		}
		config.ClientPools = append(config.ClientPools, rcp)
	}

	if cmd.Name() == "connect" {
		b := benchmark.NewConnect(config)
		err := b.Run()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		b := benchmark.New(config)
		err := b.Run()
		if err != nil {
			log.Fatal(err)
		}
	}
	if err := config.ResultRecorder.Flush(); err != nil {
		log.Fatal(err)
	}
}

func Work(cmd *cobra.Command, args []string) {
	worker := benchmark.NewWorker(options.workerListenAddr, uint16(options.workerListenPort))
	err := worker.Serve()
	if err != nil {
		log.Fatal(err)
	}
}

func parseTCPAddrs(stringAddrs []string) []*net.TCPAddr {
	var tcpAddrs []*net.TCPAddr
	for _, s := range stringAddrs {
		tcpAddrs = append(tcpAddrs, &net.TCPAddr{IP: net.ParseIP(s)})
	}

	if len(tcpAddrs) == 0 {
		tcpAddrs = []*net.TCPAddr{nil}
	}

	return tcpAddrs
}

func parseRemoteAddr(url string) (*net.TCPAddr, string, error) {
	host, port, err := net.SplitHostPort(url)
	if err != nil {
		return nil, "", err
	}

	destIPs, err := net.LookupHost(host)
	if err != nil {
		return nil, "", err
	}

	nport, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return nil, "", err
	}

	return &net.TCPAddr{IP: net.ParseIP(destIPs[0]), Port: int(nport)}, host, nil
}

func openFileWriter() (io.Writer, context.CancelFunc) {
	var err error
	dir := filepath.Dir(options.filename)
	if _, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			panic(fmt.Errorf("failed to create output dir: %v", err))
		}
	}
	file, err := os.OpenFile(options.filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		panic(fmt.Errorf("failed to open output file: %v", err))
	}
	return file, func() { _ = file.Close() }
}
