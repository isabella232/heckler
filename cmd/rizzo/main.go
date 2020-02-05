package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"

	"github.braintreeps.com/lollipopman/heckler/gitutil"
	"github.braintreeps.com/lollipopman/heckler/puppetutil"

	"google.golang.org/grpc"
)

const (
	port = ":50051"
)

// server is used to implement rizzo.RizzoServer.
type server struct {
	puppetutil.UnimplementedRizzoServer
}

// PuppetApply implements rizzo.RizzoServer
func (s *server) PuppetApply(ctx context.Context, req *puppetutil.PuppetApplyRequest) (*puppetutil.PuppetReport, error) {
	var err error
	var oid string

	log.Printf("Received: %v", req.Rev)

	// pull
	repo, err := gitutil.Pull("http://heckler:8080/muppetshow", "/var/lib/rizzo/repos/muppetshow")
	if err != nil {
		log.Printf("Pull error: %v", err)
		return &puppetutil.PuppetReport{}, err
	}
	log.Printf("Pull Complete: %v", req.Rev)

	// checkout
	oid, err = gitutil.Checkout(req.Rev, repo)
	if err != nil {
		log.Printf("Checkout error: %v", err)
		return &puppetutil.PuppetReport{}, err
	}
	log.Printf("Checkout Complete: %v", oid)

	// apply
	log.Printf("Applying: %v", oid)
	pr, err := puppetApply(oid, req.Noop)
	if err != nil {
		log.Printf("Apply error: %v", err)
		return &puppetutil.PuppetReport{}, err
	}
	log.Printf("Done: %v", req.Rev)
	return pr, nil
}

// PuppetLastApply implements rizzo.RizzoServer
func (s *server) PuppetLastApply(ctx context.Context, req *puppetutil.PuppetLastApplyRequest) (*puppetutil.PuppetReport, error) {
	var err error

	log.Printf("PuppetLastApply: request received")
	file, err := os.Open("/var/tmp/reports/heckler/heckler_last_apply.json")
	if err != nil {
		return &puppetutil.PuppetReport{}, err
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return &puppetutil.PuppetReport{}, err
	}
	pr := new(puppetutil.PuppetReport)
	err = json.Unmarshal([]byte(data), pr)
	if err != nil {
		return &puppetutil.PuppetReport{}, err
	}
	log.Printf("PuppetLastApply: status@%s", pr.ConfigurationVersion)
	return pr, nil
}

func puppetApply(oid string, noop bool) (*puppetutil.PuppetReport, error) {
	// XXX config?
	repoDir := "/var/lib/rizzo/repos/muppetshow"
	puppetArgs := []string{
		"apply",
		"--confdir",
		repoDir,
		"--vardir",
		"/var/tmp",
		"--config_version",
		"/heckler/git-head-sha",
		repoDir + "/nodes.pp",
	}
	if noop {
		puppetArgs = append(puppetArgs, "--noop")
	}
	cmd := exec.Command("puppet", puppetArgs...)
	cmd.Dir = repoDir
	stdoutStderr, err := cmd.CombinedOutput()
	log.Printf("%s", stdoutStderr)
	if err != nil {
		return &puppetutil.PuppetReport{}, err
	}
	file, err := os.Open("/var/tmp/reports/heckler/heckler_" + oid + ".json")
	if err != nil {
		return &puppetutil.PuppetReport{}, err
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return &puppetutil.PuppetReport{}, err
	}
	pr := new(puppetutil.PuppetReport)
	err = json.Unmarshal([]byte(data), pr)
	if err != nil {
		return &puppetutil.PuppetReport{}, err
	}
	return pr, nil
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	puppetutil.RegisterRizzoServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}