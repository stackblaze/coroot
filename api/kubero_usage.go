package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coroot/coroot/rca/llm"
	"k8s.io/klog"
)

const kuberoUsageTimeout = 8 * time.Second

type kuberoUsageBody struct {
	Namespace        string `json:"namespace"`
	Email            string `json:"email,omitempty"`
	Model            string `json:"model,omitempty"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	TotalTokens      int    `json:"totalTokens"`
	CachedTokens     int    `json:"cachedTokens,omitempty"`
}

// reportKuberoUsage bills incident RCA tokens to the Kubero tenant that owns
// the app namespace. Best-effort and async — a metering failure must never
// fail the analysis.
func (api *Api) reportKuberoUsage(namespace, email, model string, u llm.Usage) {
	if api == nil || api.cfg == nil {
		return
	}
	u = u.WithTotal()
	if !u.Billed() {
		return
	}
	url := strings.TrimSpace(api.cfg.Auth.KuberoUsageUrl)
	secret := api.cfg.Auth.HandoffSecret
	if url == "" || secret == "" {
		return
	}
	go func() {
		if err := postKuberoUsage(url, secret, namespace, email, model, u); err != nil {
			klog.Warningln("rca: kubero usage report failed:", err)
		}
	}()
}

func postKuberoUsage(endpoint, secret, namespace, email, model string, u llm.Usage) error {
	u = u.WithTotal()
	body, err := json.Marshal(kuberoUsageBody{
		Namespace:        namespace,
		Email:            email,
		Model:            model,
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
		CachedTokens:     u.CachedTokens,
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), kuberoUsageTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HandoffSecretHeader, secret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("kubero returned %s", resp.Status)
	}
	return nil
}
