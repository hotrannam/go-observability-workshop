package main

import (
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func workHandler(log logrus.FieldLogger) http.HandlerFunc { // pretend work
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		log = log.WithFields(logrus.Fields{
			"method":     r.Method,
			"path":       r.URL.String(),
			"request_id": id,
		})
		status := http.StatusOK // net/http returns 200 by default
		defer func(t time.Time) {
			log.WithField("status", status).WithField("duration", time.Since(t).Seconds()).Info()
		}(time.Now())

		s := rand.Intn(99) + 1 // 1..100
		log = log.WithField("s", s)

		time.Sleep(time.Duration(s) * time.Millisecond)

		switch {
		case s <= 25: // 25% of the time
			status = http.StatusInternalServerError
			log.Error("OMG Error!")
			http.Error(w, "OMG Error!", status)
		default:
			w.Write([]byte(`b = :-) `))
		}
	}
}

func slowWorkHandler(log logrus.FieldLogger) http.HandlerFunc { // slow pretend work
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		log = log.WithFields(logrus.Fields{
			"method":     r.Method,
			"path":       r.URL.String(),
			"request_id": id,
		})
		defer func(t time.Time) {
			log.WithField("status", http.StatusOK).WithField("duration", time.Since(t).Seconds()).Info()
		}(time.Now())

		s := 100 + rand.Intn(200) // 100..300
		time.Sleep(time.Duration(s) * time.Millisecond)

		w.Write([]byte(`b = 🐢 `))

	}
}

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors: true,
	})

	// curried log
	log := logrus.WithField("app", "serviceb")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	http.Handle("/metrics", promhttp.Handler())

	// Expose the port value
	info := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "program_info",
		Help: "Info about the program.",
	},
		[]string{"port"},
	)
	prometheus.MustRegister(info)
	info.WithLabelValues(port).Set(1)

	durs := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "HTTP request duration.",
			// Chosen because the range is 00-300 ms
			Buckets: []float64{.025, .05, .075, .1, .125, .15, .175, .2, .225, .250, .275, .300},
		},
		[]string{"handler", "code"},
	)
	prometheus.MustRegister(durs)
	durs.WithLabelValues("regularWork", strconv.Itoa(http.StatusOK))
	durs.WithLabelValues("regularWork", strconv.Itoa(http.StatusBadRequest))
	durs.WithLabelValues("slowWork", strconv.Itoa(http.StatusOK))
	durs.WithLabelValues("slowWork", strconv.Itoa(http.StatusBadRequest))

	http.HandleFunc("/",
		promhttp.InstrumentHandlerDuration(
			durs.MustCurryWith(prometheus.Labels{"handler": "regularWork"}),
			http.HandlerFunc(workHandler(log)),
		),
	)

	http.HandleFunc("/slow",
		promhttp.InstrumentHandlerDuration(
			durs.MustCurryWith(prometheus.Labels{"handler": "slowWork"}),
			http.HandlerFunc(slowWorkHandler(log)),
		),
	)

	log.Info("Listening at: http://localhost:" + port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Errored with: " + err.Error())
	}
}
