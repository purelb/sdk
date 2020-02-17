// Copyright (c) 2020 Cisco and/or its affiliates.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package eventchannel provides implementations based on event channels of:
//    networkservice.MonitorConnectionClient
//    networkservice.MonitorConnectionServer
//    networkservice.MonitorConnection_MonitorConnectionsClient
//    networkservice.MonitorConnection_MonitorConnectionsServer
package eventchannel

import (
	"context"

	"github.com/networkservicemesh/api/pkg/api/networkservice"
	"google.golang.org/grpc"

	"github.com/networkservicemesh/sdk/pkg/tools/serialize"
)

type monitorConnectionClient struct {
	eventCh        <-chan *networkservice.ConnectionEvent
	fanoutEventChs []chan *networkservice.ConnectionEvent
	updateExecutor serialize.Executor
}

// NewMonitorConnectionClient - returns networkservice.MonitorConnectionClient
//                              eventCh - channel that provides events to feed the Recv function
//                                        when an event is sent on the eventCh, all networkservice.MonitorConnection_MonitorConnectionsClient
//                                        returned from calling MonitorConnections receive the event.
//                              Note: Does not perform filtering basedon MonitorScopeSelector
func NewMonitorConnectionClient(eventCh <-chan *networkservice.ConnectionEvent) networkservice.MonitorConnectionClient {
	rv := &monitorConnectionClient{
		eventCh:        eventCh,
		updateExecutor: serialize.NewExecutor(),
	}
	rv.eventLoop()
	return rv
}

func (m *monitorConnectionClient) MonitorConnections(ctx context.Context, in *networkservice.MonitorScopeSelector, opts ...grpc.CallOption) (networkservice.MonitorConnection_MonitorConnectionsClient, error) {
	fanoutEventCh := make(chan *networkservice.ConnectionEvent, 100)
	m.updateExecutor.AsyncExec(func() {
		m.fanoutEventChs = append(m.fanoutEventChs, fanoutEventCh)
		go func() {
			<-ctx.Done()
			m.updateExecutor.AsyncExec(func() {
				var newFanoutEventChs []chan *networkservice.ConnectionEvent
				for _, ch := range m.fanoutEventChs {
					if ch != fanoutEventCh {
						newFanoutEventChs = append(newFanoutEventChs, ch)
					}
				}
				m.fanoutEventChs = newFanoutEventChs
				close(fanoutEventCh)
			})
		}()
	})
	return NewMonitorConnectionMonitorConnectionsClient(fanoutEventCh), nil
}

func (m *monitorConnectionClient) eventLoop() {
	go func() {
		for event := range m.eventCh {
			e := event
			m.updateExecutor.AsyncExec(func() {
				for _, fanoutEventCh := range m.fanoutEventChs {
					fanoutEventCh <- e
				}
			})
		}
	}()
}