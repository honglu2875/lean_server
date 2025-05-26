package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// REPLCommand represents a valid command to the Lean REPL
type REPLCommand struct {
	// Command mode fields
	Cmd string `json:"cmd,omitempty"`
	Env int    `json:"env,omitempty"`

	// File mode fields
	Path       string `json:"path,omitempty"`
	AllTactics bool   `json:"allTactics,omitempty"`

	// Tactic mode fields
	Tactic     string `json:"tactic,omitempty"`
	ProofState int    `json:"proofState,omitempty"`

	// Pickling fields
	PickleTo               string `json:"pickleTo,omitempty"`
	UnpickleEnvFrom        string `json:"unpickleEnvFrom,omitempty"`
	UnpickleProofStateFrom string `json:"unpickleProofStateFrom,omitempty"`
}

// REPLServer manages the Lean REPL process
type REPLServer struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mutex  sync.Mutex // Protect concurrent access to REPL
}

// NewREPLServer creates and starts a new Lean REPL process
func NewREPLServer() (*REPLServer, error) {
	// Start the lake exe repl command
	cmd := exec.Command("lake", "exe", "repl")
	if cwd := os.Getenv("REPL_PATH"); cwd != "" {
		cmd.Dir = cwd
	}

	// Get stdin pipe to write to the process
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	// Get stdout pipe to read from the process
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Create a scanner to read lines from stdout
	scanner := bufio.NewScanner(stdout)

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start REPL: %w", err)
	}

	return &REPLServer{
		cmd:    cmd,
		stdin:  stdin,
		stdout: scanner,
		mutex:  sync.Mutex{},
	}, nil
}

func depthDiff(line string, inString bool) (int, bool) {
	// Count the number of opening and closing brackets
	var count int
	var escaped bool
	for _, char := range line {
		if escaped {
			escaped = false
			continue
		}

		if char == '{' && !inString {
			count++
		} else if char == '}' && !inString {
			count--
		} else if char == '\\' && !inString {
			escaped = true
		} else if char == '"' {
			inString = !inString
		}
	}
	return count, inString
}

func Log(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the request method and URL
		log.Printf("Received request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

		// Call the next handler in the chain
		handler.ServeHTTP(w, r)

		// Log the response status
		log.Printf("Response sent for: %s %s", r.Method, r.URL.Path)
	})
}

// ExecuteCommand sends a command to the REPL and returns the response
func (s *REPLServer) ExecuteCommand(command []byte, timeout float64) ([]byte, error) {

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Send the command to the REPL followed by a blank line
	if _, err := s.stdin.Write(append(command, '\n', '\n')); err != nil {
		return nil, fmt.Errorf("failed to write to REPL: %w", err)
	}

	// Read the response from the REPL
	var responseBuilder strings.Builder
	var inResponse bool
	var inString bool
	var bracketCount int
	var diff int
	// We use a naive bracket counting strategy to determine if the response is complete. This assumes LEAN REPL does not output any escaped brackets.

	// Read until we get a non-empty response
	start_time := time.Now()
	for s.stdout.Scan() {
		// Force break after timeout
		if timeout > 0 && time.Since(start_time).Seconds() > timeout {
			break
		}

		line := s.stdout.Text()
		if !inResponse && len(line) == 0 {
			continue
		}
		if !inResponse && len(line) > 0 {
			inResponse = true
			if !strings.HasPrefix(line, "{") {
				return nil, fmt.Errorf("expecting leading curly bracket, got: %s", line)
			}
			responseBuilder.WriteString(line)
			diff, inString = depthDiff(line, inString)
			bracketCount += diff
		} else if inResponse {
			responseBuilder.WriteString(line)
			diff, inString = depthDiff(line, inString)
			bracketCount += diff
			if bracketCount <= 0 {
				break
			}
		}
	}

	if err := s.stdout.Err(); err != nil {
		return nil, fmt.Errorf("error reading from REPL: %w", err)
	}

	// Return the response
	return []byte(responseBuilder.String()), nil
}

// CleanUp properly terminates the REPL process
func (s *REPLServer) CleanUp() error {
	// Close stdin to signal EOF to the process
	if err := s.stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}

	if s.cmd.ProcessState != nil && !s.cmd.ProcessState.Exited() {
		if s.cmd.Process.Signal(os.Interrupt) != nil {
			return fmt.Errorf("failed to send interrupt signal to REPL process")
		}
	}

	// Wait for the process to exit
	if err := s.cmd.Wait(); err != nil {
		return fmt.Errorf("REPL process exited with error: %w", err)
	}

	return nil
}

func main() {
	// Create a new REPL server
	replServer, err := NewREPLServer()
	if err != nil {
		log.Fatalf("Failed to start REPL server: %v", err)
	}
	defer func() {
		if err := replServer.CleanUp(); err != nil {
			log.Printf("Error cleaning up REPL: %v", err)
		}
	}()

	// Define the port for our HTTP server
	var port int
	if portEnv := os.Getenv("PORT"); portEnv != "" {
		port, err = strconv.Atoi(portEnv)
		if err != nil {
			log.Printf("Invalid PORT environment variable, using default port 8080: %v", err)
			port = 8080
		}
	} else {
		port = 8080
	}

	// Define a timeout for each REPL command
	var timeout float64
	if timeoutEnv := os.Getenv("LEAN_REPL_TIMEOUT"); timeoutEnv != "" {
		if t, err := strconv.ParseFloat(timeoutEnv, 64); err != nil {
			timeout = t
		} else {
			timeout = -1.0
		}
	} else {
		timeout = -1.0
	}

	// Create a new HTTP server mux (router)
	mux := http.NewServeMux()

	// Register handler for the /repl endpoint
	mux.HandleFunc("/repl", func(w http.ResponseWriter, r *http.Request) {
		// Check if the request method is POST
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Read the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// Validate the JSON
		var command REPLCommand
		if err := json.Unmarshal(body, &command); err != nil {
			http.Error(w, "Invalid JSON format", http.StatusBadRequest)
			return
		}

		// Execute the command on the REPL
		response, err := replServer.ExecuteCommand(body, timeout)
		if err != nil {
			http.Error(w, fmt.Sprintf("REPL error: %v", err), http.StatusInternalServerError)
			return
		}

		// Set content type to JSON
		w.Header().Set("Content-Type", "application/json")

		// Write the response
		if _, err := w.Write(response); err != nil {
			log.Printf("Error writing response: %v", err)
		}
	})

	// Health check for /healthz endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		// Check if the REPL server is running
		if replServer.cmd.ProcessState == nil || replServer.cmd.ProcessState.Exited() {
			http.Error(w, "REPL server is not running", http.StatusInternalServerError)
			return
		}
		// Respond with a 200 OK status
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("Error writing health check response: %v", err)
		}
	})

	// Start the server
	serverAddr := fmt.Sprintf(":%d", port)
	fmt.Printf("Server starting on http://localhost%s\n", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, Log(mux)))
}
