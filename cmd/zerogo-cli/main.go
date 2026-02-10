package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/unicornultrafoundation/zerogo/internal/identity"
	"github.com/unicornultrafoundation/zerogo/internal/protocol"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	os.Args = append(os.Args[:1], os.Args[2:]...)

	switch cmd {
	case "identity":
		cmdIdentity()
	case "networks":
		cmdNetworks()
	case "members":
		cmdMembers()
	case "join":
		cmdJoin()
	case "peers":
		cmdPeers()
	case "version":
		fmt.Printf("zerogo-cli %s\n", version)
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: zerogo-cli <command> [options]

Commands:
  identity    Show or generate node identity
  networks    List/create/delete networks
  members     List/authorize/remove network members
  join        Join a network (authorize this node)
  peers       List connected peers
  version     Show version
  help        Show this help`)
}

// --- Identity command ---

func cmdIdentity() {
	fs := flag.NewFlagSet("identity", flag.ExitOnError)
	path := fs.String("identity", "/etc/zerogo/identity.key", "identity key path")
	generate := fs.Bool("generate", false, "generate new identity")
	fs.Parse(os.Args[1:])

	if *generate {
		id, err := identity.Generate()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Address:    %s\n", id.Address)
		fmt.Printf("Public Key: %s\n", id.PublicKeyHex())
		return
	}

	id, err := identity.LoadOrGenerate(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Address:    %s\n", id.Address)
	fmt.Printf("Public Key: %s\n", id.PublicKeyHex())
}

// --- Networks command ---

func cmdNetworks() {
	fs := flag.NewFlagSet("networks", flag.ExitOnError)
	controller := fs.String("controller", "http://localhost:9394", "controller URL")
	token := fs.String("token", "", "JWT auth token")
	create := fs.String("create", "", "create network with name")
	ipRange := fs.String("ip-range", "10.147.17.0/24", "IP range for new network")
	del := fs.String("delete", "", "delete network by ID")
	fs.Parse(os.Args[1:])

	client := &apiClient{base: *controller, token: *token}

	if *create != "" {
		body := protocol.CreateNetworkRequest{
			Name:    *create,
			IPRange: *ipRange,
		}
		var result protocol.Network
		if err := client.post("/api/v1/networks", body, &result); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created network: %d (%s) %s\n", result.ID, result.Name, result.IPRange)
		return
	}

	if *del != "" {
		if err := client.delete("/api/v1/networks/" + *del); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Network deleted")
		return
	}

	// List networks
	var networks []protocol.Network
	if err := client.get("/api/v1/networks", &networks); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tIP RANGE\tMEMBERS\tONLINE")
	for _, n := range networks {
		fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%d\n", n.ID, n.Name, n.IPRange, n.MemberCount, n.OnlineCount)
	}
	w.Flush()
}

// --- Members command ---

func cmdMembers() {
	fs := flag.NewFlagSet("members", flag.ExitOnError)
	controller := fs.String("controller", "http://localhost:9394", "controller URL")
	token := fs.String("token", "", "JWT auth token")
	networkID := fs.String("network", "", "network ID")
	authorize := fs.String("authorize", "", "node address to authorize")
	remove := fs.String("remove", "", "node address to remove")
	ip := fs.String("ip", "", "IP to assign when authorizing")
	fs.Parse(os.Args[1:])

	if *networkID == "" {
		fmt.Fprintln(os.Stderr, "error: --network is required")
		os.Exit(1)
	}

	client := &apiClient{base: *controller, token: *token}

	if *authorize != "" {
		body := protocol.AuthorizeMemberRequest{
			NodeAddress: *authorize,
			Authorized:  true,
			IPAddress:   *ip,
		}
		var result protocol.Member
		if err := client.post("/api/v1/networks/"+*networkID+"/members", body, &result); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Authorized: %s (IP: %s)\n", result.NodeAddress, result.IPAddress)
		return
	}

	if *remove != "" {
		if err := client.delete("/api/v1/networks/" + *networkID + "/members/" + *remove); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Member removed")
		return
	}

	// List members
	var members []protocol.Member
	if err := client.get("/api/v1/networks/"+*networkID+"/members", &members); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NODE\tIP\tAUTHORIZED\tONLINE\tPLATFORM\tLAST SEEN")
	for _, m := range members {
		lastSeen := "-"
		if !m.LastSeen.IsZero() {
			lastSeen = m.LastSeen.Format(time.RFC3339)
		}
		fmt.Fprintf(w, "%s\t%s\t%v\t%v\t%s\t%s\n",
			m.NodeAddress, m.IPAddress, m.Authorized, m.Online, m.Platform, lastSeen)
	}
	w.Flush()
}

// --- Join command ---

func cmdJoin() {
	fs := flag.NewFlagSet("join", flag.ExitOnError)
	controller := fs.String("controller", "http://localhost:9394", "controller URL")
	token := fs.String("token", "", "JWT auth token")
	networkID := fs.String("network", "", "network ID to join")
	identityPath := fs.String("identity", "/etc/zerogo/identity.key", "identity key path")
	fs.Parse(os.Args[1:])

	if *networkID == "" {
		fmt.Fprintln(os.Stderr, "error: --network is required")
		os.Exit(1)
	}

	id, err := identity.LoadOrGenerate(*identityPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading identity: %v\n", err)
		os.Exit(1)
	}

	client := &apiClient{base: *controller, token: *token}
	body := protocol.AuthorizeMemberRequest{
		NodeAddress: id.Address.String(),
		Authorized:  false, // Needs admin approval
	}
	var result protocol.Member
	if err := client.post("/api/v1/networks/"+*networkID+"/members", body, &result); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Join request sent for network %s\n", *networkID)
	fmt.Printf("Node address: %s\n", id.Address)
	fmt.Printf("Status: waiting for admin authorization\n")
}

// --- Peers command ---

func cmdPeers() {
	fs := flag.NewFlagSet("peers", flag.ExitOnError)
	controller := fs.String("controller", "http://localhost:9394", "controller URL")
	token := fs.String("token", "", "JWT auth token")
	fs.Parse(os.Args[1:])

	client := &apiClient{base: *controller, token: *token}

	var peers []json.RawMessage
	if err := client.get("/api/v1/peers", &peers); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ADDRESS\tPLATFORM\tONLINE\tLAST SEEN")
	for _, raw := range peers {
		var p struct {
			Address  string    `json:"address"`
			Platform string    `json:"platform"`
			Online   bool      `json:"online"`
			LastSeen time.Time `json:"last_seen"`
		}
		json.Unmarshal(raw, &p)
		lastSeen := "-"
		if !p.LastSeen.IsZero() {
			lastSeen = p.LastSeen.Format(time.RFC3339)
		}
		fmt.Fprintf(w, "%s\t%s\t%v\t%s\n", p.Address, p.Platform, p.Online, lastSeen)
	}
	w.Flush()
}

// --- HTTP client helper ---

type apiClient struct {
	base  string
	token string
}

func (c *apiClient) get(path string, out interface{}) error {
	req, err := http.NewRequest("GET", c.base+path, nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *apiClient) post(path string, body interface{}, out interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", c.base+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *apiClient) delete(path string) error {
	req, err := http.NewRequest("DELETE", c.base+path, nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
