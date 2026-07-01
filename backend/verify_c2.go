package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const API = "http://127.0.0.1:8080/api"

func main() {
	pass := 0
	fail := 0

	note := func(msg string) { fmt.Printf("[NOTE] %s\n", msg) }
	ok := func(msg string) { fmt.Printf("[PASS] %s\n", msg); pass++ }
	bad := func(msg string) { fmt.Printf("[FAIL] %s\n", msg); fail++ }

	login := func(email, password string) string {
		reqBody, _ := json.Marshal(map[string]string{"email": email, "password": password})
		resp, err := http.Post(API+"/auth/login", "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			bad(fmt.Sprintf("Login failed for %s: %v", email, err))
			return ""
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			bad(fmt.Sprintf("Login %s got status %d", email, resp.StatusCode))
			return ""
		}
		var out map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&out)
		return out["accessToken"].(string)
	}

	customerToken := login("cust2@test.com", "Customer12345!")
	sellerToken := login("owner2@super.com", "Seller12345!")
	adminToken := login("admin@zamk.local", "Admin12345!")

	if customerToken == "" || sellerToken == "" || adminToken == "" {
		fmt.Println("Failed to get tokens. Exiting.")
		os.Exit(1)
	}
	note("Tokens acquired successfully.")

	// Test GET /api/seller/fulfillments
	req, _ := http.NewRequest("GET", API+"/seller/fulfillments", nil)
	req.Header.Set("Authorization", "Bearer "+sellerToken)
	resp, _ := http.DefaultClient.Do(req)
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	
	if resp.StatusCode == 200 {
		ok("GET /api/seller/fulfillments -> 200")
		var out struct {
			Items []map[string]interface{} `json:"items"`
		}
		json.Unmarshal(bodyBytes, &out)
		note(fmt.Sprintf("Seller fulfillments count: %d", len(out.Items)))
		
		if len(out.Items) > 0 {
			f := out.Items[0]
			id := f["id"].(string)
			orderId := f["orderId"].(string)
			
			// Test GET /api/seller/fulfillments/{id}
			req2, _ := http.NewRequest("GET", API+"/seller/fulfillments/"+id, nil)
			req2.Header.Set("Authorization", "Bearer "+sellerToken)
			resp2, _ := http.DefaultClient.Do(req2)
			if resp2.StatusCode == 200 {
				ok("GET /api/seller/fulfillments/{id} -> 200")
			} else {
				bad(fmt.Sprintf("GET /api/seller/fulfillments/{id} -> %d", resp2.StatusCode))
			}
			resp2.Body.Close()
			
			// Test customer endpoint for this order
			reqC, _ := http.NewRequest("GET", API+"/customer/orders/"+orderId+"/fulfillments", nil)
			reqC.Header.Set("Authorization", "Bearer "+customerToken)
			respC, _ := http.DefaultClient.Do(reqC)
			bodyCBytes, _ := io.ReadAll(respC.Body)
			respC.Body.Close()
			if respC.StatusCode == 200 {
				ok("GET /api/customer/orders/{orderId}/fulfillments -> 200")
				var cFulfs []map[string]interface{}
				json.Unmarshal(bodyCBytes, &cFulfs)
				if len(cFulfs) > 0 {
					cf := cFulfs[0]
					if _, exists := cf["commissionBps"]; exists {
						bad("Customer endpoint leaked commissionBps!")
					} else {
						ok("Customer endpoint did NOT leak commissionBps")
					}
				}
			} else {
				bad(fmt.Sprintf("GET /api/customer/orders/{orderId}/fulfillments -> %d: %s", respC.StatusCode, string(bodyCBytes)))
			}
			
			// Test admin endpoints
			reqA1, _ := http.NewRequest("GET", API+"/admin/order-fulfillments", nil)
			reqA1.Header.Set("Authorization", "Bearer "+adminToken)
			respA1, _ := http.DefaultClient.Do(reqA1)
			bodyA1Bytes, _ := io.ReadAll(respA1.Body)
			respA1.Body.Close()
			if respA1.StatusCode == 200 {
				ok("GET /api/admin/order-fulfillments -> 200")
			} else {
				bad(fmt.Sprintf("GET /api/admin/order-fulfillments -> %d: %s", respA1.StatusCode, string(bodyA1Bytes)))
			}
			
			reqA2, _ := http.NewRequest("GET", API+"/admin/orders/"+orderId+"/fulfillments", nil)
			reqA2.Header.Set("Authorization", "Bearer "+adminToken)
			respA2, _ := http.DefaultClient.Do(reqA2)
			bodyA2Bytes, _ := io.ReadAll(respA2.Body)
			respA2.Body.Close()
			if respA2.StatusCode == 200 {
				ok("GET /api/admin/orders/{orderId}/fulfillments -> 200")
			} else {
				bad(fmt.Sprintf("GET /api/admin/orders/{orderId}/fulfillments -> %d: %s", respA2.StatusCode, string(bodyA2Bytes)))
			}
		}
	} else {
		bad(fmt.Sprintf("GET /api/seller/fulfillments -> %d", resp.StatusCode))
	}
	
	// Check Old APIs
	reqOld1, _ := http.NewRequest("GET", API+"/customer/orders", nil)
	reqOld1.Header.Set("Authorization", "Bearer "+customerToken)
	respOld1, _ := http.DefaultClient.Do(reqOld1)
	if respOld1.StatusCode == 200 { ok("GET /api/customer/orders -> 200") } else { bad("GET /api/customer/orders failed") }
	respOld1.Body.Close()

	reqOld2, _ := http.NewRequest("GET", API+"/seller/orders", nil)
	reqOld2.Header.Set("Authorization", "Bearer "+sellerToken)
	respOld2, _ := http.DefaultClient.Do(reqOld2)
	if respOld2.StatusCode == 200 { ok("GET /api/seller/orders -> 200") } else { bad("GET /api/seller/orders failed") }
	respOld2.Body.Close()

	fmt.Printf("\n=== SUMMARY: %d passed, %d failed ===\n", pass, fail)
	if fail > 0 {
		os.Exit(1)
	}
}
