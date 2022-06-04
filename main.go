package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	promVersion "github.com/prometheus/common/version"
)

const (
	maxDatagramSize = 8192
)

func init() {
	promVersion.Version = "0.1.0"
	prometheus.MustRegister(promVersion.NewCollector("sma_home_manager_exporter"))
}

func main() {
	var (
		listenAddr   = flag.String("web.listen-address", ":9192", "The address to listen on for HTTP requests.")
		multicastAddr = flag.String("multicast.addr", "239.12.255.254:9522", "Multicast address to listen on.")
		pollInterval = flag.Int("inverter.poll-interval", 5, "Interval in seconds between polls.")
		showVersion  = flag.Bool("version", false, "Print version information and exit.")
	)

	flag.Parse()

	if *showVersion {
		fmt.Printf("%s\n", promVersion.Print("sma_home_manager_exporter"))
		os.Exit(0)
	}

	var shmOutWatts = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "shm_out_watts",
		Help: "SMA Home Manager 2.0 outbound power (sell)",
	})
	var shmInWatts = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "shm_in_watts",
		Help: "SMA Home Manager 2.0 inbound power (buy)",
	})

	// Register the summary and the histogram with Prometheus's default registry
	prometheus.MustRegister(shmOutWatts)
	prometheus.MustRegister(shmInWatts)

	// Add Go module build info
	prometheus.MustRegister(collectors.NewBuildInfoCollector())

	// Poll inverter values
	go func() {

		// Parse the string address
		addr, err := net.ResolveUDPAddr("udp4", *multicastAddr)
		if err != nil {
			log.Fatal(err)
		}

		// Open up a connection
		conn, err := net.ListenMulticastUDP("udp4", nil, addr)
		if err != nil {
			log.Fatal(err)
		}

		conn.SetReadBuffer(maxDatagramSize)

		// Loop forever reading from the socket
		for {
			buffer := make([]byte, maxDatagramSize)
			_, _, err := conn.ReadFromUDP(buffer)
			if err != nil {
				log.Fatal("ReadFromUDP failed:", err)
			}

			// TODO: Check this
			// buffer[16:18] == '\x60\x69'
			// buffer[0:3] == "SMA"

			// TODO: function to convert byte range to float
			totalPowerIn := buffer[34:36]
			totalPowerOut := buffer[54:56]
			out := float64(binary.BigEndian.Uint16(totalPowerOut)) * 0.1
			in := float64(binary.BigEndian.Uint16(totalPowerIn)) * 0.1

			shmInWatts.Set(in)
			shmOutWatts.Set(out)

			time.Sleep(time.Duration(*pollInterval) * time.Second)
		}
	}()

	// Expose the registered metrics via HTTP
	http.Handle("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{},
	))
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
