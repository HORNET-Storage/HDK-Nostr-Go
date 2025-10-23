package main

import (
	//"context"
	//"encoding/json"

	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	merkle_dag "github.com/HORNET-Storage/Scionic-Merkle-Tree/v2/dag"
	"github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	"github.com/HORNET-Storage/go-hornet-storage-lib/lib/connmgr"
	"github.com/HORNET-Storage/go-hornet-storage-lib/lib/signing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/nbd-wtf/go-nostr"
)

// These are example keys generated for the purpose of this test client
// Please do not use them for anything other than this
const npub string = "npub128qpftun6mzv3gh7pfcsq3sefgqwruz43kuhhu78hq99jeak5a0sstnu23"
const nsec string = "nsec1yas03jagdjsr8su00g92jurf7am3dldvu9tckyz796z8efpa594qp2nelz"

func main() {
	ctx := context.Background()

	RunCommandWatcher(ctx)
}

func RunCommandWatcher(ctx context.Context) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan

		Cleanup(ctx)
		os.Exit(0)
	}()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		scanner.Scan()

		command := strings.TrimSpace(scanner.Text())
		segments := strings.Split(command, " ")

		switch segments[0] {
		case "help":
			log.Println("Available Commands:")
			log.Println("upload")
			log.Println("download")
			log.Println("event")
			log.Println("keys")
			log.Println("shutdown")
		case "upload":
			UploadDag(ctx, segments[1])
		case "download":
			DownloadDag(ctx, segments[1])
		case "query":
			QueryDag()
		case "event":
			UploadEvent(ctx)
		case "keys":
			TestKeys(ctx)
		case "shutdown":
			log.Println("Shutting down")
			Cleanup(ctx)
			return
		default:
			log.Printf("Unknown command: %s\n", command)
		}
	}
}

func Cleanup(ctx context.Context) {

}

func UploadDag(ctx context.Context, path string) {
	// Create a new dag from a directory
	dag, err := merkle_dag.CreateDag(path, false)
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}

	// Verify the entire dag
	err = dag.Verify()
	if err != nil {
		log.Fatalf("Error: %s", err)
	}

	log.Println("Dag verified correctly")

	publicKey, err := signing.DeserializePublicKey(npub)
	if err != nil {
		log.Fatal(err)
	}

	_, pubKeyTest, err := signing.DeserializePrivateKey(nsec)
	if err != nil {
		log.Fatal(err)
	}

	test1, err := signing.SerializePublicKey(pubKeyTest)
	if err != nil {
		log.Fatal(err)
	}

	test2, err := signing.SerializePublicKey(publicKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(*test1)
	fmt.Println(*test2)

	libp2pPubKey, err := signing.ConvertPubKeyToLibp2pPubKey(publicKey)
	if err != nil {
		log.Fatal(err)
	}

	peerId, err := peer.IDFromPublicKey(*libp2pPubKey)
	if err != nil {
		log.Fatal(err)
	}

	conMgr := connmgr.NewGenericConnectionManager()

	err = conMgr.ConnectWithLibp2p(ctx, "default", fmt.Sprintf("/ip4/127.0.0.1/udp/9000/quic-v1/p2p/%s", peerId.String()), libp2p.Transport(libp2pquic.NewTransport))
	if err != nil {
		log.Fatal(err)
	}

	jsonData, _ := dag.ToJSON()
	os.WriteFile("before_upload.json", jsonData, 0644)

	privateKey, _, err := signing.DeserializePrivateKey(nsec)
	if err != nil {
		log.Fatal(err)
	}

	progressChan := make(chan lib.UploadProgress)

	go func() {
		for progress := range progressChan {
			if progress.Error != nil {
				fmt.Printf("Error uploading to %s: %v\n", progress.ConnectionID, progress.Error)
			} else {
				fmt.Printf("Progress for %s: %d/%d leafs uploaded\n", progress.ConnectionID, progress.LeafsSent, progress.TotalLeafs)
			}
		}
	}()

	// Upload the dag to the hornet storage node
	err = connmgr.UploadDagSingle(ctx, conMgr, "default", dag, privateKey, progressChan)
	if err != nil {
		log.Fatal(err)
	}

	close(progressChan)

	conMgr.Disconnect("default")
}

func DownloadDag(ctx context.Context, root string) {
	// Connect to a hornet storage node
	publicKey, err := signing.DeserializePublicKey(npub)
	if err != nil {
		log.Fatal(err)
	}

	libp2pPubKey, err := signing.ConvertPubKeyToLibp2pPubKey(publicKey)
	if err != nil {
		log.Fatal(err)
	}

	peerId, err := peer.IDFromPublicKey(*libp2pPubKey)
	if err != nil {
		log.Fatal(err)
	}

	conMgr := connmgr.NewGenericConnectionManager()

	err = conMgr.ConnectWithLibp2p(ctx, "default", fmt.Sprintf("/ip4/127.0.0.1/udp/9000/quic-v1/p2p/%s", peerId.String()), libp2p.Transport(libp2pquic.NewTransport))
	if err != nil {
		log.Fatal(err)
	}

	progressChan := make(chan lib.DownloadProgress)

	go func() {
		for progress := range progressChan {
			if progress.Error != nil {
				fmt.Printf("Error uploading to %s: %v\n", progress.ConnectionID, progress.Error)
			} else {
				fmt.Printf("Progress for %s: %d leafs downloaded\n", progress.ConnectionID, progress.LeafsRetreived)
			}
		}
	}()

	_, dag, err := connmgr.DownloadDag(ctx, conMgr, "default", root, nil, nil, progressChan)
	if err != nil {
		log.Fatal(err)
	}

	close(progressChan)

	jsonData, _ := json.Marshal(dag.Dag.ToSerializable())
	os.WriteFile("after_download.json", jsonData, 0644)

	conMgr.Disconnect("default")
}

func QueryDag() {
	ctx := context.Background()

	// Connect to a hornet storage node
	publicKey, err := signing.DeserializePublicKey(npub)
	if err != nil {
		log.Fatal(err)
	}

	libp2pPubKey, err := signing.ConvertPubKeyToLibp2pPubKey(publicKey)
	if err != nil {
		log.Fatal(err)
	}

	peerId, err := peer.IDFromPublicKey(*libp2pPubKey)
	if err != nil {
		log.Fatal(err)
	}

	conMgr := connmgr.NewGenericConnectionManager()

	err = conMgr.ConnectWithLibp2p(ctx, "default", fmt.Sprintf("/ip4/127.0.0.1/udp/9000/quic-v1/p2p/%s", peerId.String()), libp2p.Transport(libp2pquic.NewTransport))
	if err != nil {
		log.Fatal(err)
	}

	query := types.QueryFilter{
		Tags: map[string]string{
			"repo_id": "51c014af93d6c4c8a2fe0a710046194a00e1f0558db97bf3c7b80a5967b6a75f:nestr",
		},
	}

	// Upload the dag to the hornet storage node
	hashes, err := connmgr.QueryDag(ctx, conMgr, "default", query)
	if err != nil {
		log.Fatal(err)
	}

	data, err := json.Marshal(hashes)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("RESULTS")

	for _, hash := range hashes {
		fmt.Println(hash)
	}

	fmt.Println(data)

	if len(hashes) > 0 {
		for _, hash := range hashes {
			_, dag, err := connmgr.DownloadDag(ctx, conMgr, "default", hash, nil, nil, nil)
			if err != nil {
				log.Fatal(err)
			}

			jsonData, _ := json.Marshal(dag.Dag.ToSerializable())
			os.WriteFile(fmt.Sprintf("%s.json", hash), jsonData, 0644)
		}
	}

	// Disconnect client as we no longer need it
	conMgr.Disconnect("default")
}

func UploadEvent(ctx context.Context) {
	// Construct content for metadata event
	metadataContent := map[string]string{
		"name":    "TestName",
		"about":   "TestAbout",
		"picture": "TestPicture",
	}

	// Serialize content to JSON
	contentBytes, err := json.Marshal(metadataContent)
	if err != nil {
		log.Fatal(err)
	}
	content := string(contentBytes)

	// Create event object
	event := &nostr.Event{
		PubKey:    npub,
		CreatedAt: nostr.Now(),
		Kind:      0,
		Tags: []nostr.Tag{
			{"e", "5c83da77af1dec6d7289834998ad7aafbd9e2191396d75ec3cc27f5a77226f36", "wss://nostr.example.com"},
			{"p", "f7234bd4c1394dda46d09f35bd384dd30cc552ad5541990f98844fb06676e9ca"},
		},
		Content: content,
	}

	event.Sign(signing.TrimPrivateKey(nsec))

	success, err := event.CheckSignature()
	if err != nil {
		log.Fatal(err)
	}

	if !success {
		log.Fatal(fmt.Errorf("failed to verify event signature"))
	}

	log.Println("Event sorted")

	// Connect to a hornet storage node
	publicKey, err := signing.DeserializePublicKey(npub)
	if err != nil {
		log.Fatal(err)
	}

	libp2pPubKey, err := signing.ConvertPubKeyToLibp2pPubKey(publicKey)
	if err != nil {
		log.Fatal(err)
	}

	peerId, err := peer.IDFromPublicKey(*libp2pPubKey)
	if err != nil {
		log.Fatal(err)
	}

	conMgr := connmgr.NewGenericConnectionManager()

	err = conMgr.ConnectWithLibp2p(ctx, "default", fmt.Sprintf("/ip4/127.0.0.1/udp/9000/quic-v1/p2p/%s", peerId.String()), libp2p.Transport(libp2pquic.NewTransport))
	if err != nil {
		log.Fatal(err)
	}

	results, err := connmgr.SendUniversalEvent(ctx, conMgr, event, nil)
	if err != nil {
		log.Fatal(err)
	}

	okEnv, ok := results["default"]
	if !ok {
		log.Fatal("Did not get a valid response")
	}

	fmt.Println(okEnv.EventID + " | " + okEnv.Reason)

	filter := nostr.Filter{
		IDs: []string{okEnv.EventID},
	}

	subId := "default"
	events, err := connmgr.QueryEvents(ctx, conMgr, "default", []nostr.Filter{filter}, &subId)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Logging events")

	for _, event := range events {
		fmt.Printf("Found Event %s of kind %d", event.ID, event.Kind)
	}
}

func TestKeys(ctx context.Context) {
	// Connect to a hornet storage node
	publicKey, err := signing.DeserializePublicKey(npub)
	if err != nil {
		log.Fatal(err)
	}

	libp2pPubKey, err := signing.ConvertPubKeyToLibp2pPubKey(publicKey)
	if err != nil {
		log.Fatal(err)
	}

	peerId, err := peer.IDFromPublicKey(*libp2pPubKey)
	if err != nil {
		log.Fatal(err)
	}

	privKey, pubKey, err := signing.DeserializePrivateKey(nsec)
	if err != nil {
		log.Fatal(err)
	}

	spr, err := signing.SerializePrivateKey(privKey)
	if err != nil {
		log.Fatal(err)
	}

	sp, err := signing.SerializePublicKey(pubKey)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("private (hex): " + *spr)
	log.Println("public (hex): " + *sp)

	log.Printf("Keys deserialized properly and your libp2p peer id is: %s\n", peerId)
}

func TimeCheck(eventCreatedAt int64) (bool, string) {
	const timeCutoff = 5 * time.Minute // Define your own cutoff threshold
	eventTime := time.Unix(eventCreatedAt, 0)

	// Check if the event timestamp is too far in the past or future
	if time.Since(eventTime) > timeCutoff || time.Until(eventTime) > timeCutoff {
		errMsg := fmt.Sprintf("invalid: event creation date is too far off from the current time (%s)", eventTime)
		return false, errMsg
	}
	return true, ""
}
