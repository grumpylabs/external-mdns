// Copyright 2020 Blake Covarrubias
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Portions refactored by :-
//    Copyright (c) 2025 Robert B. Gordon
//    Licensed under the MIT License.
//    https://opensource.org/licenses/MIT

package cmd

import (
	"fmt"
	"log"

	"net"
	"time"

	"github.com/grumpylabs/external-mdns/cmd/config"
	"github.com/grumpylabs/external-mdns/cmd/mdns"
	"github.com/grumpylabs/external-mdns/cmd/mdns/resource"
	"github.com/grumpylabs/external-mdns/cmd/source"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/spf13/viper"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
)

var (
	svcCmd = &cobra.Command{
		Use:   "svc",
		Short: "Start the external-mDNS service",
		Run:   run,
	}
	lg *zap.Logger
)

func init() {
	rootCmd.AddCommand(svcCmd)

	// Flags
	svcCmd.Flags().Bool(config.Debug, false, "Enable debug logging")
	svcCmd.Flags().String(config.KubeConfig, "", "(optional) Absolute path to the kubeconfig file")
	svcCmd.Flags().String(config.Master, "", "URL to Kubernetes master")
	svcCmd.Flags().String(config.Namespace, "", "Limit sources of endpoints to a specific namespace")
	svcCmd.Flags().Bool(config.PublishInternalServices, false, "Publish ClusterIP services")
	svcCmd.Flags().Bool(config.Test, false, "Run in testing mode (no connection to Kubernetes)")
	svcCmd.Flags().Int(config.RecordTTL, 120, "DNS record TTL")
	svcCmd.Flags().Bool(config.WithoutNamespace, false, "Publish shorter mDNS names without namespace")
	svcCmd.Flags().StringSlice(config.Source, []string{"service"}, "Resource types to query (options: service, ingress)")
	svcCmd.Flags().Bool(config.ExposeIPv4, true, "Publish IPv4 addresses")
	svcCmd.Flags().Bool(config.ExposeIPv6, false, "Publish IPv6 addresses")
	svcCmd.Flags().String(config.DefaultNamespace, "default", "Default namespace to use if not specified in the resource")

	// Bind Cobra flags to Viper
	viper.BindPFlags(svcCmd.Flags())
}

type k8sSource []string

func (s *k8sSource) String() string {
	return fmt.Sprint(*s)
}

func (s *k8sSource) Set(value string) error {
	switch value {
	case "ingress", "service":
		*s = append(*s, value)
	}
	return nil
}

/*
The following functions were obtained from
https://gist.github.com/trajber/7cb6abd66d39662526df

  - hexDigit
  - reverseAddress()
*/
const hexDigit = "0123456789abcdef"

func reverseAddress(addr string) (arpa string, err error) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return "", &net.DNSError{Err: "unrecognized address", Name: addr}
	}
	if ip.To4() != nil {
		return net.IPv4(ip[15], ip[14], ip[13], ip[12]).String() + ".in-addr.arpa.", nil
	}
	// Must be IPv6
	buf := make([]byte, 0, len(ip)*4+len("ip6.arpa."))
	// Add it, in reverse, to the buffer
	for i := len(ip) - 1; i >= 0; i-- {
		v := ip[i]
		buf = append(buf, hexDigit[v&0xF])
		buf = append(buf, '.')
		buf = append(buf, hexDigit[v>>4])
		buf = append(buf, '.')
	}
	// Append "ip6.arpa." and return (buf already has the final .)
	buf = append(buf, "ip6.arpa."...)
	return string(buf), nil
}

func constructRecords(r resource.Resource) []string {
	var records []string

	for _, resourceIP := range r.IPs {
		ip := net.ParseIP(resourceIP)
		if ip == nil {
			continue
		}

		reverseIP, _ := reverseAddress(resourceIP)

		var recordType string
		if ip.To4() != nil {
			if !viper.GetBool(config.ExposeIPv4) {
				continue
			}
			recordType = "A"
		} else {
			if !viper.GetBool(config.ExposeIPv6) {
				continue
			}
			recordType = "AAAA"
		}

		// Publish records resources as <name>.<namespace>.local and as <name>-<namespace>.local
		// Because Windows does not support subdomains resolution via mDNS and uses regular DNS query instead.
		// Ensure corresponding PTR records map to this hostname
		// To maintain backwards compatibility, without-namespace annontation still generates these records
		for _, name := range r.Names {
			records = append(records, fmt.Sprintf("%s.%s.local. %d IN %s %s", name, r.Namespace, viper.GetInt(config.RecordTTL), recordType, ip))
			records = append(records, fmt.Sprintf("%s-%s.local. %d IN %s %s", name, r.Namespace, viper.GetInt(config.RecordTTL), recordType, ip))
			if reverseIP != "" {
				records = append(records, fmt.Sprintf("%s %d IN PTR %s.%s.local.", reverseIP, viper.GetInt(config.RecordTTL), name, r.Namespace))
				records = append(records, fmt.Sprintf("%s %d IN PTR %s-%s.local.", reverseIP, viper.GetInt(config.RecordTTL), name, r.Namespace))
			}
		}

		// Publish services without the name in the namespace if any of the following
		// criteria is satisfied:
		// 1. The Service exists in the default namespace
		// 2. Service names exposed with annotation and with additional without-namespace annotation set to true
		// 3. The -without-namespace flag is equal to true
		// 4. The record to be published is from an Ingress with a defined hostname
		if r.Namespace == viper.GetString(config.DefaultNamespace) || r.WithoutNamespace || viper.GetBool(config.WithoutNamespace) || r.SourceType == "ingress" {
			for _, name := range r.Names {
				records = append(records, fmt.Sprintf("%s.local. %d IN %s %s", name, viper.GetInt(config.RecordTTL), recordType, ip))
				if reverseIP != "" {
					records = append(records, fmt.Sprintf("%s %d IN PTR %s.local.", reverseIP, viper.GetInt(config.RecordTTL), name))
				}
			}
		}
	}

	return records
}

func publishRecord(rr string) {
	if err := mdns.Publish(rr); err != nil {
		lg.Fatal("Failed to publish record ", zap.String("record", rr), zap.Error(err))
	}
}

func unpublishRecord(rr string) {
	if err := mdns.UnPublish(rr); err != nil {
		lg.Fatal("Failed to unpublish record ", zap.String("record", rr), zap.Error(err))
	}
}

// Run the service
func run(cmd *cobra.Command, args []string) {
	var (
		lg  *zap.Logger
		err error
	)

	if lg, err = NewLogger(); err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	// Print configuration
	lg.Debug("Starting external-mDNS with configuration:",
		zap.Any("settings", viper.AllSettings()))

	if viper.GetBool("test") {
		publishRecord("router.local. 60 IN A 192.168.1.254")
		publishRecord("254.1.168.192.in-addr.arpa. 60 IN PTR router.local.")
		select {}
	}

	sources := viper.GetStringSlice("source")
	if len(sources) == 0 {
		lg.Fatal("Error: No sources specified. Use --source=service or --source=ingress.")
	}

	k8sClient, err := newK8sClient()
	if err != nil {
		lg.Fatal("Failed to create Kubernetes client:", zap.Error(err))
	}

	notifyMdns := make(chan resource.Resource)
	stopper := make(chan struct{})
	defer close(stopper)
	defer runtime.HandleCrash()

	factory := informers.NewSharedInformerFactory(k8sClient, time.Minute*5)

	for _, src := range sources {
		switch src {
		case "ingress":
			ingressController := source.NewIngressWatcher(lg, factory, viper.GetString(config.Namespace), notifyMdns)
			go ingressController.Run(stopper)
		case "service":
			serviceController := source.NewServicesWatcher(
				lg,
				factory,
				viper.GetString(config.Namespace),
				notifyMdns,
				viper.GetBool(config.PublishInternalServices),
			)
			go serviceController.Run(stopper)
		}
	}

	for {
		select {
		case advertiseResource := <-notifyMdns:
			for _, record := range constructRecords(advertiseResource) {
				if record == "" {
					continue
				}
				switch advertiseResource.Action {
				case resource.Added:
					lg.Info("Publishing new DNS record:", zap.String("record", record))
					publishRecord(record)
				case resource.Deleted:
					lg.Info("Removing DNS record:", zap.String("record", record))
					unpublishRecord(record)
				}
			}
		case <-stopper:
			lg.Info("Stopping external-mdns")
			return
		}
	}
}
