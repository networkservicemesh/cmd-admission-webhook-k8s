package watcher

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	certsFileMode = os.FileMode(0o644)
	keyFileMode   = os.FileMode(0o600)
	certsFileName = "tls.crt"
	keyFileName   = "tls.key"
	socketPath    = "unix:///run/spire/sockets/agent.sock"
	bName         = "notexistingfile"
)

func WatchForCertificateUpdate(ctx context.Context, certDir string) error {
	log.Print("WatchForCertificateUpdate")
	client, err := workloadapi.New(ctx, workloadapi.WithAddr(socketPath))
	if err != nil {
		return fmt.Errorf("unable to create workload api client %w", err)
	}

	go func() {
		defer client.Close()
		err := client.WatchX509Context(ctx, &x509Watcher{CertDir: certDir})
		if err != nil && status.Code(err) != codes.Canceled {
			log.Fatalf("Error watching X.509 context: %v", err)
		}
	}()
	if err = waitForCertificates(certDir); err != nil {
		return err
	}
	return nil
}

// waitForCertificates waits up to 3 minutes for the certificate, key, and bundle
// to be on disk.
func waitForCertificates(dir string) error {
	// log.Print("waitForCertificates: wait")
	certsFile := path.Join(dir, certsFileName)
	keyFile := path.Join(dir, keyFileName)

	log.Printf("key: %v", keyFile)
	log.Printf("file: %v", certsFile)

	sleep := 500 * time.Millisecond
	maxRetries := 360
	for i := 1; i <= maxRetries; i++ {
		log.Print("waitForCertificates: wait")
		time.Sleep(sleep)

		if _, err := os.Stat(certsFile); err != nil {
			continue
		}

		if _, err := os.Stat(keyFile); err != nil {
			continue
		}

		return nil
	}

	return errors.New("timed out waiting for trust bundle")
}

// WriteToDisk takes a X509SVIDResponse, representing a svid message from the Workload API
// and write the certs to disk
func writeToDisk(svid *x509svid.SVID, dir string) error {
	log.Print("writeToDisk: try to write")
	certsFile := path.Join(dir, certsFileName)
	keyFile := path.Join(dir, keyFileName)

	pemCerts, pemKey, err := svid.Marshal()
	if err != nil {
		log.Print("Marshal")
		log.Print(fmt.Errorf("unable to marshal X.509 SVID: %w", err))
		return fmt.Errorf("unable to marshal X.509 SVID: %w", err)
	}

	if err := ioutil.WriteFile(certsFile, pemCerts, certsFileMode); err != nil {
		log.Print("cert")
		log.Print(fmt.Errorf("error writing certs file: %w", err))
		return fmt.Errorf("error writing certs file: %w", err)
	}

	if err := ioutil.WriteFile(keyFile, pemKey, keyFileMode); err != nil {
		log.Print("key")
		log.Print(fmt.Errorf("error writing key file: %w", err))
		return fmt.Errorf("error writing key file: %w", err)
	}

	log.Print("Successfully wrote")
	return nil
}

type x509Watcher struct {
	CertDir string
}

func (watcher *x509Watcher) OnX509ContextUpdate(ctx *workloadapi.X509Context) {
	log.Printf("OnX509ContextUpdate: Update called for dir %v", watcher.CertDir)
	writeToDisk(ctx.DefaultSVID(), watcher.CertDir)
}

func (watcher *x509Watcher) OnX509ContextWatchError(err error) {
	log.Print("OnX509ContextWatchError: Watch error called")
	if status.Code(err) != codes.Canceled {
		log.Printf("OnX509ContextWatcherError error: %v", err)
	}
}
