/*
Copyright 2021 The Dapr Authors
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

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/dapr/go-sdk/dapr/proto/runtime/v1"
	pb "github.com/dapr/go-sdk/dapr/proto/runtime/v1"
	"github.com/dapr/go-sdk/service/grpc"
	daprd "github.com/dapr/go-sdk/service/grpc"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func main() {
	ctx := context.Background()

	go startServer()

	time.Sleep(5 * time.Second)

	// create the client
	client, err := dapr.NewClient()
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// Invoke actor with ID "123"
	res, err := client.GrpcClient().InvokeActorV2Alpha1(ctx, &runtime.InvokeActorV2Alpha1Request{
		AppId:     "dev",
		ActorType: "myactor",
		ActorId:   "123",
		Method:    "hello",
		Data:      []byte("first call"),
		Metadata: map[string]string{
			"foo": "bar",
		},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("Response:", string(res.Data))

	time.Sleep(2 * time.Second)

	wg := sync.WaitGroup{}
	wg.Add(3)

	// Invoke actor with ID "123" twice in parallel
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			res, err := client.GrpcClient().InvokeActorV2Alpha1(ctx, &runtime.InvokeActorV2Alpha1Request{
				AppId:     "dev",
				ActorType: "myactor",
				ActorId:   "123",
				Method:    "hello",
				Data:      []byte("ciao mondo 1/" + strconv.Itoa(i)),
				Metadata: map[string]string{
					"foo": "bar",
				},
			})
			if err != nil {
				panic(err)
			}
			fmt.Println("Response:", string(res.Data))

			time.Sleep(2 * time.Second)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			res, err := client.GrpcClient().InvokeActorV2Alpha1(ctx, &runtime.InvokeActorV2Alpha1Request{
				AppId:     "dev",
				ActorType: "myactor",
				ActorId:   "123",
				Method:    "hello",
				Data:      []byte("ciao mondo 2/" + strconv.Itoa(i)),
				Metadata: map[string]string{
					"foo": "bar",
				},
			})
			if err != nil {
				panic(err)
			}
			fmt.Println("Response:", string(res.Data))

			time.Sleep(2 * time.Second)
		}
	}()

	// Invoke a different actor
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			res, err := client.GrpcClient().InvokeActorV2Alpha1(ctx, &runtime.InvokeActorV2Alpha1Request{
				AppId:     "dev",
				ActorType: "myactor",
				ActorId:   "456",
				Method:    "hello",
				Data:      []byte("hello world " + strconv.Itoa(i)),
				Metadata: map[string]string{
					"foo": "bar",
				},
			})
			if err != nil {
				panic(err)
			}
			fmt.Println("Response:", string(res.Data))

			time.Sleep(2 * time.Second)
		}
	}()

	wg.Wait()
}

func startServer() {
	s, err := daprd.NewService(":9001")
	if err != nil {
		log.Fatalf("error creating service: %v", err)
	}

	grpcSrv := s.(*grpc.Server).GrpcServer()
	pb.RegisterAppCallbackAlphaServer(grpcSrv, &alphaSrv{})

	err = s.Start()
	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("error listenning: %v", err)
	}
}

type alphaSrv struct {
	pb.UnimplementedAppCallbackAlphaServer
}

// Invokes an actor using the Actors v2 APIs
func (s *alphaSrv) OnActorInvokeV2(ctx context.Context, in *pb.ActorInvokeV2Request) (*pb.ActorInvokeV2Response, error) {
	fmt.Println("Request with data: '"+string(in.Data.Value)+"' State:", in.State)
	time.Sleep(2 * time.Second)
	newState, _ := structpb.NewStruct(map[string]any{
		"date": time.Now().UTC().String(),
	})
	return &pb.ActorInvokeV2Response{
		Data: &anypb.Any{
			Value: []byte("pong"),
		},
		State: &pb.ActorInvokeV2Response_Set{
			Set: &pb.ActorInvokeV2Response_SetActorState{
				State: newState,
			},
		},
	}, nil
}
