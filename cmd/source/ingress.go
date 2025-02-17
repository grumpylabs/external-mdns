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

package source

import (
	"fmt"
	"strings"

	"github.com/grumpylabs/external-mdns/cmd/mdns/resource"
	"github.com/jpillora/go-tld"
	"go.uber.org/zap"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// IngressSource handles adding, updating, or removing mDNS record advertisements
type IngressSource struct {
	lg             *zap.Logger
	namespace      string
	notifyChan     chan<- resource.Resource
	sharedInformer cache.SharedIndexInformer
}

// Run starts shared informers and waits for the shared informer cache to
// synchronize.
func (i *IngressSource) Run(stopCh chan struct{}) error {
	i.sharedInformer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, i.sharedInformer.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
	}
	return nil
}

func (i *IngressSource) onAdd(obj interface{}) {
	advertiseRecords, err := i.buildRecords(obj, resource.Added)

	if err != nil {
		i.lg.Info("Error adding ingress", zap.Error(err), zap.Any("ingress", obj))
		return
	}

	for _, record := range advertiseRecords {
		i.notifyChan <- record
	}
}

func (i *IngressSource) onDelete(obj interface{}) {
	advertiseRecords, err := i.buildRecords(obj, resource.Deleted)

	if err != nil {
		i.lg.Info("Error deleting ingress", zap.Error(err), zap.Any("ingress", obj))
		return
	}

	for _, record := range advertiseRecords {
		i.notifyChan <- record
	}
}

func (i *IngressSource) onUpdate(oldObj interface{}, newObj interface{}) {
	oldResources, err1 := i.buildRecords(oldObj, resource.Updated)
	if err1 != nil {
		i.lg.Info("Error gathering old ingress resources", zap.Error(err1), zap.Any("ingress", oldObj))
	}

	for _, record := range oldResources {
		record.Action = resource.Deleted
		i.notifyChan <- record
	}

	newResources, err2 := i.buildRecords(newObj, resource.Updated)
	if err2 != nil {
		i.lg.Info("Error gathering new ingress resources", zap.Error(err2), zap.Any("ingress", newObj))
	}

	for _, record := range newResources {
		record.Action = resource.Added
		i.notifyChan <- record
	}
}

func (i *IngressSource) buildRecords(obj interface{}, action string) ([]resource.Resource, error) {
	var records []resource.Resource

	ingress, ok := obj.(*v1.Ingress)
	if !ok {
		return records, nil
	}

	var ipFields []string
	for _, lb := range ingress.Status.LoadBalancer.Ingress {
		if lb.IP != "" {
			ipFields = append(ipFields, lb.IP)
		}
	}

	if len(ipFields) == 0 {
		return records, nil
	}

	// Advertise each hostname under this Ingress
	var hostname string
	for _, rule := range ingress.Spec.Rules {
		// Skip rules with no hostname or that do not use the .local TLD
		if rule.Host == "" || !strings.HasSuffix(rule.Host, ".local") {
			continue
		}

		fakeURL := fmt.Sprintf("http://%s", rule.Host)
		parsedHost, err := tld.Parse(fakeURL)

		if err != nil {
			i.lg.Info("Unable to parse hostname", zap.Error(err), zap.Any("hostname", rule.Host))
			continue
		}

		if parsedHost.Subdomain != "" {
			hostname = fmt.Sprintf("%s.%s", parsedHost.Subdomain, parsedHost.Domain)
		} else {
			hostname = parsedHost.Domain
		}
		advertiseObj := resource.Resource{
			SourceType: "ingress",
			Action:     action,
			Names:      []string{hostname},
			Namespace:  ingress.Namespace,
			IPs:        ipFields,
		}

		records = append(records, advertiseObj)
	}
	return records, nil
}

// NewIngressWatcher creates an IngressSource
func NewIngressWatcher(lg *zap.Logger, factory informers.SharedInformerFactory, namespace string, notifyChan chan<- resource.Resource) IngressSource {
	ingressInformer := factory.Networking().V1().Ingresses().Informer()
	i := &IngressSource{
		lg:             lg,
		namespace:      namespace,
		notifyChan:     notifyChan,
		sharedInformer: ingressInformer,
	}

	ingressInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    i.onAdd,
		DeleteFunc: i.onDelete,
		UpdateFunc: i.onUpdate,
	})

	return *i
}
