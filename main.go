package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/andygrunwald/megos"
	"github.com/hashicorp/consul/api"
)

type configOptions struct {
	Master      []*url.URL
	Frameworks  []string
	Interval    time.Duration
	Healthcheck string
}

type addressPort struct {
	Host string
	Port int
}

var config = &configOptions{}

var mesosLeader *addressPort
var frameworkLeaders = make(map[string]*addressPort)

func initConfig() {
	nodes := flag.String("nodes", "127.0.0.1:5050", "comma-separated list of Mesos nodes")
	frameworks := flag.String("frameworks", "marathon,chronos", "comma-separated list of Mesos frameworks to discover (eg. 'marathon,chronos')")
	flag.DurationVar(&config.Interval, "duration", 10*time.Second, "interval at which to check for topology change")
	flag.StringVar(&config.Healthcheck, "healthcheck", ":8080", "listen address of healthcheck HTTP server")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})
	go http.ListenAndServe(config.Healthcheck, nil)

	for _, node := range strings.Split(*nodes, ",") {
		url, err := url.Parse(fmt.Sprintf("http://%s", node))
		if err != nil {
			logrus.Error(err)
			continue
		}
		config.Master = append(config.Master, url)
	}

	config.Frameworks = strings.Split(*frameworks, ",")
}

func main() {
	initConfig()

	mesos := megos.NewClient(config.Master)

	for {
		time.Sleep(config.Interval)

		wg := sync.WaitGroup{}

		leader, err := mesos.DetermineLeader()
		if err != nil {
			logrus.Errorf("could not determine Mesos leader: %s", err)
			continue
		}

		go func() {
			wg.Add(1)
			defer wg.Done()

			newMesosLeader := &addressPort{Host: leader.Host, Port: leader.Port}

			if !reflect.DeepEqual(newMesosLeader, mesosLeader) {
				c := api.DefaultConfig()
				c.Address = fmt.Sprintf("%s:8500", newMesosLeader.Host)
				client, _ := api.NewClient(c)

				logrus.Infof("new Mesos leader detected: %s:%d", newMesosLeader.Host, newMesosLeader.Port)

				err := client.Agent().ServiceRegister(&api.AgentServiceRegistration{
					ID:      "mesos:master",
					Name:    "mesos",
					Address: newMesosLeader.Host,
					Port:    newMesosLeader.Port,
					Tags:    []string{"master"},
					Check: &api.AgentServiceCheck{
						Interval: "10s",
						Timeout:  "30s",
						TCP:      fmt.Sprintf("%s:%d", newMesosLeader.Host, newMesosLeader.Port),
					},
				})

				if err != nil {
					logrus.Errorf("could not register Mesos master: %s", err)
				} else {
					mesosLeader = newMesosLeader
				}
			}
		}()

		state, err := mesos.GetStateFromLeader()
		if err != nil {
			logrus.Errorf("could not get state: %s", err)
			continue
		}

		for _, f := range config.Frameworks {
			go func(f string) {
				wg.Add(1)
				defer wg.Done()
				framework, err := mesos.GetFrameworkByPrefix(state.Frameworks, f)
				if err != nil {
					logrus.Errorf("could not retrieve framwork '%s': %s", "marathon", err)
					return
				}

				url, err := url.Parse(framework.WebuiURL)
				if err != nil {
					logrus.Errorf("could not parse framework URL (%s): %s", framework.WebuiURL, err)
					return
				}
				host, port, err := net.SplitHostPort(url.Host)
				if err != nil {
					logrus.Errorf("could not split host and port from '%s': %s", url.Host, err)
					return
				}
				p, err := strconv.Atoi(port)
				if err != nil {
					logrus.Errorf("could not parse port number '%s': %s", port, err)
					return
				}

				newFrameworkLeader := &addressPort{Host: host, Port: p}

				if !reflect.DeepEqual(newFrameworkLeader, frameworkLeaders[f]) {
					c := api.DefaultConfig()
					c.Address = fmt.Sprintf("%s:8500", newFrameworkLeader.Host)
					client, _ := api.NewClient(c)

					logrus.Infof("new framework leader for '%s' detected: %s:%d", f, newFrameworkLeader.Host, newFrameworkLeader.Port)

					err := client.Agent().ServiceRegister(&api.AgentServiceRegistration{
						ID:      fmt.Sprintf("mesos:framework:%s", f),
						Name:    "mesos",
						Address: host,
						Port:    p,
						Tags:    []string{f},
						Check: &api.AgentServiceCheck{
							Interval: "10s",
							Timeout:  "30s",
							TCP:      fmt.Sprintf("%s:%d", host, p),
						},
					})

					if err != nil {
						logrus.Errorf("could not register framework '%s' master: %s", f, err)
					} else {
						frameworkLeaders[f] = newFrameworkLeader
					}
				}
			}(f)
		}

		wg.Wait()
	}
}
