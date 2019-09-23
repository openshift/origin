//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package metrics

import (
	"log"
	"net/http"

	"github.com/heketi/heketi/apps"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	app apps.Application
}

const (
	namespace = "heketi"
)

var (
	up = promDesc(
		"up",
		"Is heketi running?",
		nil,
	)

	clusterCount = promDesc(
		"cluster_count",
		"Number of clusters",
		nil,
	)

	volumesCount = promDesc(
		"volumes_count",
		"Number of volumes on cluster",
		[]string{"cluster"},
	)

	nodesCount = promDesc(
		"nodes_count",
		"Number of nodes on cluster",
		[]string{"cluster"},
	)

	deviceCount = promDesc(
		"device_count",
		"Number of devices on host",
		[]string{"cluster", "hostname"},
	)

	deviceSize = promDesc(
		"device_size",
		"Total size of the device",
		[]string{"cluster", "hostname", "device"},
	)

	deviceFree = promDesc(
		"device_free",
		"Amount of Free space available on the device",
		[]string{"cluster", "hostname", "device"},
	)

	deviceUsed = promDesc(
		"device_used",
		"Amount of space used on the device",
		[]string{"cluster", "hostname", "device"},
	)

	brickCount = promDesc(
		"device_brick_count",
		"Number of bricks on device",
		[]string{"cluster", "hostname", "device"},
	)
)

func promDesc(name, help string, variableLabels []string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", name),
		help,
		variableLabels,
		nil,
	)
}

// Describe all the metrics exported by Heketi exporter. It implements prometheus.Collector.
func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- clusterCount
	ch <- volumesCount
	ch <- nodesCount
	ch <- deviceCount
	ch <- deviceSize
	ch <- deviceFree
	ch <- deviceUsed
	ch <- brickCount

}

// Collect metrics from heketi app
func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	topinfo, err := m.app.TopologyInfo()

	upVal := 0.0
	if err == nil {
		upVal = 1.0
	}

	ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, upVal)

	//Do not collect further metrics if heketi is down
	if err != nil {
		log.Println("Can't collect toplogy info for metrics: " + err.Error())
		return
	}

	ch <- prometheus.MustNewConstMetric(
		clusterCount,
		prometheus.GaugeValue,
		float64(len(topinfo.ClusterList)),
	)
	for _, cluster := range topinfo.ClusterList {
		ch <- prometheus.MustNewConstMetric(
			volumesCount,
			prometheus.GaugeValue,
			float64(len(cluster.Volumes)),
			cluster.Id,
		)
		ch <- prometheus.MustNewConstMetric(
			nodesCount,
			prometheus.GaugeValue,
			float64(len(cluster.Nodes)),
			cluster.Id,
		)
		for _, node := range cluster.Nodes {
			ch <- prometheus.MustNewConstMetric(
				deviceCount,
				prometheus.GaugeValue,
				float64(len(node.DevicesInfo)),
				cluster.Id,
				node.Hostnames.Manage[0],
			)
			for _, device := range node.DevicesInfo {
				ch <- prometheus.MustNewConstMetric(
					deviceSize,
					prometheus.GaugeValue,
					float64(device.Storage.Total),
					cluster.Id,
					node.Hostnames.Manage[0],
					device.Name,
				)
				ch <- prometheus.MustNewConstMetric(
					deviceFree,
					prometheus.GaugeValue,
					float64(device.Storage.Free),
					cluster.Id,
					node.Hostnames.Manage[0],
					device.Name,
				)
				ch <- prometheus.MustNewConstMetric(
					deviceUsed, prometheus.GaugeValue,
					float64(device.Storage.Used),
					cluster.Id,
					node.Hostnames.Manage[0],
					device.Name,
				)
				ch <- prometheus.MustNewConstMetric(
					brickCount,
					prometheus.GaugeValue,
					float64(len(device.Bricks)),
					cluster.Id,
					node.Hostnames.Manage[0],
					device.Name,
				)
			}
		}
	}
}

func NewMetricsHandler(app apps.Application) http.HandlerFunc {
	m := &Metrics{
		app: app,
	}
	prometheus.MustRegister(m)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		promhttp.Handler().ServeHTTP(w, r)
	})
}
