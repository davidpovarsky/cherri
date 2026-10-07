/*
 * Copyright (c) Cherri
 */

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/electrikmilk/args-parser"
	"howett.net/plist"
)

var signFailed = false
var signingServiceFailed = false
var backoff = 10

// SignShortcut signs the unsigned shortcut at inPath and writes the signed file to outPath.
func SignShortcut(inPath, outPath, mode string) error {
	if mode == "" {
		mode = "people-who-know-me"
		if args.Using("share") && args.Value("share") == "anyone" {
			mode = "anyone"
		}
	}

	inputPath = inPath
	outputPath = outPath

	if darwin {
		if args.Using("debug") {
			fmt.Printf("Signing %s to %s...", inPath, outPath)
		}
		var signCmd = exec.Command(
			"shortcuts",
			"sign",
			"-i", inPath,
			"-o", outPath,
			"-m", mode,
		)
		var stdErr bytes.Buffer
		signCmd.Stderr = &stdErr
		var signErr = signCmd.Run()
		if signErr == nil {
			data, readErr := os.ReadFile(outPath)
			if readErr == nil && looksLikeSignedShortcut(data) {
				if args.Using("debug") {
					fmt.Println(ansi("Done.", green))
				}
				return nil
			}
		}

		signFailed = true
		if args.Using("debug") {
			fmt.Print(ansi("Failed!\n", red))
		}
		fmt.Printf("%s\n%s\n", ansi("Failed to sign Shortcut using macOS :(", orange, bold), ansi(stdErr.String(), orange))
	}

	var hub = hubSign()
	return useSigningServiceExplicit(&hub, inPath, outPath)
}

// sign runs the shortcuts sign command on the unsigned shortcut file.
func sign() {
	var signingMode = "people-who-know-me"
	if args.Using("share") && args.Value("share") == "anyone" {
		signingMode = "anyone"
	}
	if err := SignShortcut(inputPath, outputPath, signingMode); err != nil {
		exit(err.Error())
	}
}

func useHubSign() {
	var hubSignService = hubSign()
	if err := useSigningServiceExplicit(&hubSignService, inputPath, outputPath); err != nil {
		exit(err.Error())
	}
}

type SigningService struct {
	name string
	url  string
	info func() string
}

// Sign the Shortcut using a signing service.
func useSigningService(service *SigningService) {
	if err := useSigningServiceExplicit(service, inputPath, outputPath); err != nil {
		exit(err.Error())
	}
}

func useSigningServiceExplicit(service *SigningService, inPath, outPath string) error {
	handleBackoff(service)

	if !args.Using("no-ansi") {
		fmt.Println(ansi(fmt.Sprintf("Signing using %s service...", service.name), green))
		if service.info != nil {
			fmt.Println(service.info())
		}
	}

	var signedShortcut, err = requestSignedShortcutExplicit(service, inPath, outPath)
	if err != nil {
		return err
	}
	if len(signedShortcut) == 0 {
		return fmt.Errorf("signing service returned empty response")
	}

	if !looksLikeSignedShortcut(signedShortcut) {
		return fmt.Errorf("signing server response does not look like a signed Shortcut (missing AEA1 magic)")
	}

	var writeErr = os.WriteFile(outPath, signedShortcut, 0644)
	if writeErr != nil {
		return fmt.Errorf("failed to write signed shortcut to %s: %w", outPath, writeErr)
	}

	if args.Using("debug") {
		fmt.Println(ansi("Done.", green))
	}
	return nil
}

func handleBackoff(service *SigningService) {
	if signingServiceFailed {
		fmt.Println(ansi(fmt.Sprintf("Backing off from %s", service.name), red))
		for i := 5; i > 0; i-- {
			fmt.Printf("%d seconds...\r", i)
			time.Sleep(1 * time.Second)
		}
		fmt.Print("\n\n")
	}
}

func requestSignedShortcut(service *SigningService) []byte {
	bytes, err := requestSignedShortcutExplicit(service, inputPath, outputPath)
	if err != nil {
		exit(err.Error())
	}
	return bytes
}

func requestSignedShortcutExplicit(service *SigningService, inPath, outPath string) ([]byte, error) {
	var xmlData []byte
	var marshalErr error

	if len(shortcut.WFWorkflowActions) > 0 {
		xmlData, marshalErr = plist.Marshal(shortcut, plist.XMLFormat)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed to marshal shortcut plist: %w", marshalErr)
		}
	} else if inPath != "" {
		raw, readErr := os.ReadFile(inPath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read unsigned shortcut %s: %w", inPath, readErr)
		}
		xmlData = raw
	} else {
		return nil, fmt.Errorf("no shortcut data or input path available to sign")
	}

	name := basename
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(outPath), ".shortcut")
	}
	if name == "" {
		name = "Shortcut"
	}

	var payload = map[string]string{
		"shortcutName": name,
		"shortcut":     string(xmlData),
	}
	var jsonPayload, jsonErr = json.Marshal(payload)
	if jsonErr != nil {
		return nil, fmt.Errorf("failed to marshal json payload: %w", jsonErr)
	}

	var request, httpErr = http.NewRequest("POST", service.url, bytes.NewReader(jsonPayload))
	if httpErr != nil {
		return nil, fmt.Errorf("failed to create http request: %w", httpErr)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", fmt.Sprintf("cherri/%s", version))

	var client = &http.Client{
		Timeout: time.Second * 30,
	}
	var response, resErr = client.Do(request)
	if resErr != nil {
		return nil, fmt.Errorf("signing request error: %w", resErr)
	}
	defer response.Body.Close()

	var responseContentType = response.Header.Get("Content-Type")
	var allowedContentTypes = []string{"application/octet-stream", "application/x-plist", "application/x-apple-shortcut"}
	if !slices.Contains(allowedContentTypes, responseContentType) {
		return nil, fmt.Errorf("unsupported response type: %s", responseContentType)
	}

	if response.StatusCode != http.StatusOK {
		signingServiceFailed = true
		backoff += 10
		return nil, fmt.Errorf("failed to sign Shortcut (%s)", response.Status)
	}

	signingServiceFailed = false

	if backoff > 10 {
		backoff -= 10
	} else {
		backoff = 10
	}

	var body, readErr = io.ReadAll(response.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read response body: %w", readErr)
	}

	return body, nil
}

// looksLikeSignedShortcut performs quick checks to make sure response is a signed Shortcut.
func looksLikeSignedShortcut(buffer []byte) bool {
	if len(buffer) >= 4 && string(buffer[:4]) == "AEA1" {
		return true
	}
	return false
}

func removeUnsigned() {
	var _, signedStatErr = os.Stat(fmt.Sprintf("%s%s.shortcut", relativePath, workflowName))
	if os.IsNotExist(signedStatErr) {
		return
	}
	var _, unsignedStatErr = os.Stat(fmt.Sprintf("%s%s%s", relativePath, workflowName, unsignedEnd))
	if os.IsNotExist(unsignedStatErr) {
		return
	}

	if args.Using("debug") {
		fmt.Printf("Removing %s%s...", workflowName, unsignedEnd)
	}

	var removeErr = os.Remove(inputPath)
	handle(removeErr)

	if args.Using("debug") {
		fmt.Println(ansi("Done.", green))
	}
}

// Sign the Shortcut using RoutineHub's signing service.
func hubSign() SigningService {
	return SigningService{
		name: "HubSign",
		url:  "https://hubsign.routinehub.services/sign",
		info: func() string {
			return fmt.Sprintf("Shortcut Signing Powered By %s", ansi("RoutineHub", red))
		},
	}
}
