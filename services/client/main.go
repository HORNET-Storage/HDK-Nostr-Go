package main

import (
	//"context"
	//"encoding/json"

	"bufio"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	merkle_dag "github.com/HORNET-Storage/scionic-merkletree/dag"
	"github.com/multiformats/go-multibase"

	"github.com/HORNET-Storage/go-hornet-storage-lib/lib/connmgr"
	hornet_rsa "github.com/HORNET-Storage/go-hornet-storage-lib/lib/encryption/rsa"
)

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
			log.Println("generate")
			log.Println("parse")
			log.Println("upload")
			log.Println("download")
			log.Println("shutdown")
		case "generate":
			GenerateKeys(ctx)
		case "parse":
			ParseKeys(ctx)
		case "upload":
			UploadDag(ctx, segments[1])
		case "download":
			DownloadDag(ctx, segments[1])
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
	dag, err := merkle_dag.CreateDag(path, multibase.Base64)
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}

	// Get the encoder based on the dag root
	encoder := multibase.MustNewEncoder(multibase.Base64)

	// Verify the entire dag
	result, err := dag.Verify(encoder)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}

	if result {
		log.Println("Dag verified correctly")
	} else {
		log.Fatal("Dag failed to verify")
	}

	// Connect to a hornet storage node
	publicKey := "12D3KooWK5w15heWibLQ7KUeKvVwbq8dTaSmad9FxaVD6jtUCT3j"

	ctx, client, err := connmgr.Connect(ctx, "0.0.0.0", "9000", publicKey)
	if err != nil {
		log.Fatal(err)
	}

	jsonData, _ := dag.ToJSON()
	os.WriteFile("before_upload.json", jsonData, 0644)

	//IterateDag(dag, func(leaf *merkle_dag.DagLeaf) {
	//	log.Printf("Processing leaf: %s\n", leaf.Hash)
	//})

	// Upload the dag to the hornet storage node
	ctx, err = client.UploadDag(ctx, dag, nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Disconnect client as we no longer need it
	client.Disconnect()
}

func DownloadDag(ctx context.Context, root string) {
	// Connect to a hornet storage node
	publicKey := "12D3KooWK5w15heWibLQ7KUeKvVwbq8dTaSmad9FxaVD6jtUCT3j"

	ctx, client, err := connmgr.Connect(ctx, "0.0.0.0", "9000", publicKey)
	if err != nil {
		log.Fatal(err)
	}

	// Upload the dag to the hornet storage node
	ctx, dag, err := client.DownloadDag(ctx, root, nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	encoding, _, err := multibase.Decode(dag.Root)
	if err != nil {
		return
	}

	encoder := multibase.MustNewEncoder(encoding)

	// Verify the entire dag
	result, err := dag.Verify(encoder)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}

	if result {
		log.Println("Dag verified correctly")
	} else {
		log.Fatal("Dag failed to verify")
	}

	jsonData, _ := json.Marshal(dag)
	os.WriteFile("after_download.json", jsonData, 0644)

	err = dag.CreateDirectory("D:/organizations/akashic_record/relevant/golang/output", encoder)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}

	// Disconnect client as we no longer need it
	client.Disconnect()
}

func GenerateKeys(ctx context.Context) {
	privateKey, err := hornet_rsa.CreateKeyPair()
	if err != nil {
		fmt.Println("Failed to create private key")
		return
	}

	hornet_rsa.SaveKeyPairToFile(privateKey)
}

func ParseKeys(ctx context.Context) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := hornet_rsa.ParsePrivateKeyFromFile("private.key")
	if err != nil {
		fmt.Println("Failed to parse private key")
		return nil, nil, err
	}

	publicKey, err := hornet_rsa.ParsePublicKeyFromFile("public.pem")
	if err != nil {
		fmt.Println("Failed to parse public key")
		return nil, nil, err
	}

	log.Printf("Private Key: %s\n", privateKey)
	log.Printf("Public Key: %s\n", publicKey)

	return privateKey, publicKey, nil
}
