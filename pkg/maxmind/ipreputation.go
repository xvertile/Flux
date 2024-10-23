package ipreputation

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "sync"
    "time"
)

var (
    ip2LocationKey = "0DBE3E93EB7479CC586F1640330BE892"
    cache         = make(map[string]ReputationData)
    cacheMutex    sync.RWMutex
    cacheTimeout  = 24 * time.Hour
)

type ReputationData struct {
    IsProxy      bool      `json:"is_proxy"`
    ProxyType    string    `json:"proxy_type"`
    VPNScore     float64   `json:"vpn_score"`
    LastChecked  time.Time `json:"last_checked"`
    ProxyProvider string   `json:"proxy_provider"`
}

type IP2LocationResponse struct {
    IsProxy bool   `json:"is_proxy"`
    ProxyType string `json:"proxy_type"`
    Provider string `json:"provider"`
}

func CheckIPReputation(ip string) (ReputationData, error) {
    // Check cache first
    cacheMutex.RLock()
    if data, exists := cache[ip]; exists && time.Since(data.LastChecked) < cacheTimeout {
        cacheMutex.RUnlock()
        return data, nil
    }
    cacheMutex.RUnlock()

    // Make concurrent requests to both services
    var wg sync.WaitGroup
    var ip2locData IP2LocationResponse
    var getipintelScore float64
    var ip2locErr, getipintelErr error

    wg.Add(2)

    // IP2Location check
    go func() {
        defer wg.Done()
        ip2locData, ip2locErr = checkIP2Location(ip)
    }()

    // GetIPIntel check
    go func() {
        defer wg.Done()
        getipintelScore, getipintelErr = checkGetIPIntel(ip)
    }()

    wg.Wait()

    if ip2locErr != nil && getipintelErr != nil {
        return ReputationData{}, fmt.Errorf("all reputation checks failed")
    }

    // Combine results
    result := ReputationData{
        IsProxy:      ip2locData.IsProxy || getipintelScore > 0.99,
        ProxyType:    ip2locData.ProxyType,
        VPNScore:     getipintelScore,
        LastChecked:  time.Now(),
        ProxyProvider: ip2locData.Provider,
    }

    // Update cache
    cacheMutex.Lock()
    cache[ip] = result
    cacheMutex.Unlock()

    return result, nil
}

func checkIP2Location(ip string) (IP2LocationResponse, error) {
    url := fmt.Sprintf("https://api.ip2location.io/?key=%s&ip=%s", ip2LocationKey, ip)
    resp, err := http.Get(url)
    if err != nil {
        return IP2LocationResponse{}, err
    }
    defer resp.Body.Close()

    var result IP2LocationResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return IP2LocationResponse{}, err
    }

    return result, nil
}

func checkGetIPIntel(ip string) (float64, error) {
    email := "contact@example.com" // You should rotate this
    url := fmt.Sprintf("http://check.getipintel.net/check.php?ip=%s&contact=%s&flags=m", ip, email)
    
    resp, err := http.Get(url)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()

    var body string
    if _, err := fmt.Fscanf(resp.Body, "%s", &body); err != nil {
        return 0, err
    }

    score, err := strconv.ParseFloat(body, 64)
    if err != nil {
        return 0, err
    }

    if score < 0 {
        return 0, fmt.Errorf("GetIPIntel error: %f", score)
    }

    return score, nil
}