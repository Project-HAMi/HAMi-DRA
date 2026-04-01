/*
Copyright 2026 The HAMi Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package configmapgen

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"

	"k8s.io/klog/v2"
)

type BootstrapData struct {
	DefaultNamespace string       `json:"defaultNamespace"`
	DefaultName      string       `json:"defaultName"`
	DefaultKey       string       `json:"defaultKey"`
	Form             FormDefaults `json:"form"`
}

type FormDefaults struct {
	NodeSelectorKey   string                 `json:"nodeSelectorKey"`
	NodeSelectorValue string                 `json:"nodeSelectorValue"`
	GroupTemplate     CollectionDefaults     `json:"groupTemplate"`
	NodeTemplate      NodeCollectionConfig   `json:"nodeTemplate"`
	Groups            []CollectionDefaults   `json:"groups"`
	Nodes             []NodeCollectionConfig `json:"nodes"`
}

type CollectionDefaults struct {
	Name                     string `json:"name"`
	SelectorKey              string `json:"selectorKey"`
	SelectorValue            string `json:"selectorValue"`
	DeviceCount              int    `json:"deviceCount"`
	DeviceNamePrefix         string `json:"deviceNamePrefix"`
	MinorStart               int    `json:"minorStart"`
	ProductName              string `json:"productName"`
	Memory                   string `json:"memory"`
	Cores                    string `json:"cores"`
	Architecture             string `json:"architecture"`
	Brand                    string `json:"brand"`
	DeviceType               string `json:"deviceType"`
	CUDACapability           string `json:"cudaComputeCapability"`
	CUDADriverVersion        string `json:"cudaDriverVersion"`
	DriverVersion            string `json:"driverVersion"`
	PCIeRoot                 string `json:"pcieRoot"`
	PCIeBusStart             string `json:"pcieBusStart"`
	AllowMultipleAllocations bool   `json:"allowMultipleAllocations"`
}

type NodeCollectionConfig struct {
	NodeName string `json:"nodeName"`
	CollectionDefaults
}

func StartServer(ctx context.Context, addr string) (*http.Server, error) {
	handler, err := NewHandler()
	if err != nil {
		return nil, err
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			klog.ErrorS(err, "web server stopped unexpectedly", "addr", addr)
		}
	}()

	return server, nil
}

func NewHandler() (http.Handler, error) {
	staticFS, err := fs.Sub(webAssets, "web/static")
	if err != nil {
		return nil, fmt.Errorf("load static assets: %w", err)
	}

	indexHTML, err := webAssets.ReadFile("web/index.html")
	if err != nil {
		return nil, fmt.Errorf("load index.html: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))
	mux.HandleFunc("/api/defaults", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(defaultBootstrapData())
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux, nil
}

func defaultBootstrapData() BootstrapData {
	return BootstrapData{
		DefaultNamespace: "default",
		DefaultName:      "fake-dra-config",
		DefaultKey:       "config.yaml",
		Form: FormDefaults{
			NodeSelectorKey:   "node-role.kubernetes.io/worker",
			NodeSelectorValue: "",
			GroupTemplate: CollectionDefaults{
				Name:                     "new-group",
				SelectorKey:              "gpu-type",
				SelectorValue:            "a100",
				DeviceCount:              2,
				DeviceNamePrefix:         "gpu",
				MinorStart:               0,
				ProductName:              "NVIDIA A100-SXM4-80GB",
				Memory:                   "80Gi",
				Cores:                    "100",
				Architecture:             "Ampere",
				Brand:                    "Nvidia",
				DeviceType:               "hami-gpu",
				CUDACapability:           "8.0.0",
				CUDADriverVersion:        "12.9.0",
				DriverVersion:            "575.57.8",
				PCIeRoot:                 "pci0000:5a",
				PCIeBusStart:             "61",
				AllowMultipleAllocations: true,
			},
			NodeTemplate: NodeCollectionConfig{
				NodeName: "worker-1",
				CollectionDefaults: CollectionDefaults{
					Name:                     "node-override",
					DeviceCount:              1,
					DeviceNamePrefix:         "gpu",
					MinorStart:               0,
					ProductName:              "NVIDIA A100-SXM4-80GB",
					Memory:                   "80Gi",
					Cores:                    "100",
					Architecture:             "Ampere",
					Brand:                    "Nvidia",
					DeviceType:               "hami-gpu",
					CUDACapability:           "8.0.0",
					CUDADriverVersion:        "12.9.0",
					DriverVersion:            "575.57.8",
					PCIeRoot:                 "pci0000:5a",
					PCIeBusStart:             "61",
					AllowMultipleAllocations: true,
				},
			},
			Groups: []CollectionDefaults{
				{
					Name:                     "a100",
					SelectorKey:              "gpu-type",
					SelectorValue:            "a100",
					DeviceCount:              2,
					DeviceNamePrefix:         "gpu",
					MinorStart:               0,
					ProductName:              "NVIDIA A100-SXM4-80GB",
					Memory:                   "80Gi",
					Cores:                    "100",
					Architecture:             "Ampere",
					Brand:                    "Nvidia",
					DeviceType:               "hami-gpu",
					CUDACapability:           "8.0.0",
					CUDADriverVersion:        "12.9.0",
					DriverVersion:            "575.57.8",
					PCIeRoot:                 "pci0000:5a",
					PCIeBusStart:             "61",
					AllowMultipleAllocations: true,
				},
			},
			Nodes: []NodeCollectionConfig{
				{
					NodeName: "worker-1",
					CollectionDefaults: CollectionDefaults{
						Name:                     "worker-1-override",
						DeviceCount:              1,
						DeviceNamePrefix:         "gpu",
						MinorStart:               2,
						ProductName:              "NVIDIA A100-SXM4-80GB",
						Memory:                   "80Gi",
						Cores:                    "100",
						Architecture:             "Ampere",
						Brand:                    "Nvidia",
						DeviceType:               "hami-gpu",
						CUDACapability:           "8.0.0",
						CUDADriverVersion:        "12.9.0",
						DriverVersion:            "575.57.8",
						PCIeRoot:                 "pci0000:5a",
						PCIeBusStart:             "63",
						AllowMultipleAllocations: true,
					},
				},
			},
		},
	}
}
