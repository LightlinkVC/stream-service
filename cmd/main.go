package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/mux"
	roomDelivery "github.com/lightlink/stream-service/pkg/room/application/delivery/http"
	"github.com/lightlink/stream-service/pkg/room/application/eventhandler"
	"github.com/lightlink/stream-service/pkg/room/application/roommanager"
	"github.com/lightlink/stream-service/pkg/room/infrastructure/repository/inmemory"
	"github.com/lightlink/stream-service/pkg/room/infrastructure/rpc/kurento"
	"github.com/lightlink/stream-service/pkg/room/infrastructure/ws/centrifugo"
	proto "github.com/lightlink/stream-service/protogen/rtpproxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	centrifugoKey := os.Getenv("CENTRIFUGO_HTTP_API_KEY")

	centrifugoClient := centrifugo.NewCentrifugoClient(
		"http://centrifugo:8000",
		centrifugoKey,
	)

	roomRepository := inmemory.NewInMemoryRoomRepository(
		&sync.Map{},
		&sync.Map{},
	)

	client, _ := grpc.Dial(
		fmt.Sprintf("%s:%s", os.Getenv("RTP_PROXY_SERVICE_GRPC_HOST"), os.Getenv("RTP_PROXY_SERVICE_GRPC_PORT")),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	rtpProxyClient := proto.NewRtpProxyServiceClient(client)

	eventHandler := eventhandler.NewDefaultEventHandler(centrifugoClient, roomRepository)

	kurentoClient, err := kurento.NewKurentoClient(
		fmt.Sprintf(
			"ws://%s:%s/kurento",
			os.Getenv("KURENTO_MEDIA_SERVER_HOST"),
			os.Getenv("KURENTO_MEDIA_SERVER_PORT"),
		),
		eventHandler,
	)
	if err != nil {
		panic(err)
	}

	roomManager := roommanager.NewDefaultRoomManager(
		kurentoClient,
		roomRepository,
		centrifugoClient,
		rtpProxyClient,
	)

	roomHandler := roomDelivery.NewRoomHandler(roomManager, centrifugoClient)

	router := mux.NewRouter()

	router.HandleFunc("/api/room/{roomID}", roomHandler.JoinHandler).Methods("POST")
	router.HandleFunc("/api/room/{roomID}/info", roomHandler.InfoHandler).Methods("GET")
	router.HandleFunc("/api/room/{roomID}/signal", roomHandler.SingalHandler).Methods("POST")
	router.HandleFunc("/ws/api/room/{roomID}/trace", roomHandler.ConnectionStateHandler).Methods("GET")

	log.Println("starting server at http://127.0.0.1:8085")
	log.Fatal(http.ListenAndServe(":8085", router))
}
