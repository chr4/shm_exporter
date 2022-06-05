package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
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
		listenAddr    = flag.String("web.listen-address", ":9192", "The address to listen on for HTTP requests.")
		multicastAddr = flag.String("multicast.addr", "239.12.255.254:9522", "Multicast address to listen on.")
		showVersion   = flag.Bool("version", false, "Print version information and exit.")
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
		for {
			// Sleep so we don't retry obsessively on failures
			time.Sleep(time.Second)

			// Parse the string address
			addr, err := net.ResolveUDPAddr("udp4", *multicastAddr)
			if err != nil {
				log.Println(err)
				continue
			}

			// Open up a connection
			conn, err := net.ListenMulticastUDP("udp4", nil, addr)
			if err != nil {
				log.Println(err)
				continue
			}

			err = conn.SetReadBuffer(maxDatagramSize)
			if err != nil {
				log.Println(err)
				err = conn.Close()
				if err != nil {
					log.Println(err)
				}
				continue
			}

			// Loop forever reading from the socket
			for {
				// Packets should come in around once a second. If we don't receive one for 5s, assume
				// the connection is dead and restart
				err = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				if err != nil {
					log.Println(err)
					break
				}

				buffer := make([]byte, maxDatagramSize)
				_, _, err := conn.ReadFromUDP(buffer)
				if err != nil {
					log.Println("ReadFromUDP failed:", err)
					break
				}

				// Check whether some magic codes are present to verify this is a message from SHM
				if bytes.Compare(buffer[0:3], []byte{'S', 'M', 'A'}) != 0 {
					continue
				}
				if bytes.Compare(buffer[16:18], []byte{'\x60', '\x69'}) != 0 {
					continue
				}

				totalPowerIn := bufferToFloat(buffer[34:36])
				totalPowerOut := bufferToFloat(buffer[54:56])

				shmInWatts.Set(totalPowerIn)
				shmOutWatts.Set(totalPowerOut)
			}

			err = conn.Close()
			if err != nil {
				log.Println(err)
			}
		}
	}()

	// Expose the registered metrics via HTTP
	http.Handle("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{},
	))
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}

// Convert 16 bit buffer to float64 (Watts require a factor of 0.1)
func bufferToFloat(b []byte) float64 {
	return float64(binary.BigEndian.Uint16(b)) * 0.1
}
