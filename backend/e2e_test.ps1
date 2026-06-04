$ErrorActionPreference = "Stop"

function Invoke-Api {
    param (
        [string]$Method,
        [string]$Endpoint,
        [hashtable]$Body,
        [string]$Token
    )
    
    $headers = @{}
    if ($Token) {
        $headers["Authorization"] = "Bearer $Token"
    }

    $uri = "http://localhost:8080$Endpoint"
    
    $params = @{
        Method = $Method
        Uri = $uri
        Headers = $headers
    }
    
    if ($Body) {
        $params.Body = ($Body | ConvertTo-Json -Depth 10)
        $params.ContentType = "application/json"
    }
    
    try {
        $response = Invoke-RestMethod @params -SkipHttpErrorCheck -StatusCodeVariable "statusCode"
        return @{ Status = $statusCode; Data = $response }
    } catch {
        return @{ Status = 500; Data = $_.Exception.Response }
    }
}

Write-Host "1. Logging in as Admin..."
$adminLogin = Invoke-Api -Method "POST" -Endpoint "/api/auth/login" -Body @{ email = "admin@zamk.com"; password = "securePassword123" }
if ($adminLogin.Status -ne 200) { throw "Admin login failed: $($adminLogin.Status)" }
$adminToken = $adminLogin.Data.accessToken
Write-Host "Admin logged in successfully."

Write-Host "2. Creating a Seller..."
$sellerReq = @{
    brandName = "Super Store"
    description = "A great store"
    contactEmail = "store@super.com"
    contactPhone = "+123456789"
    ownerName = "Store Owner"
    ownerEmail = "owner@super.com"
    temporaryPassword = "storePassword123"
}
$createSeller = Invoke-Api -Method "POST" -Endpoint "/api/admin/sellers" -Body $sellerReq -Token $adminToken
if ($createSeller.Status -ne 201) { throw "Create seller failed: $($createSeller.Status) - $($createSeller.Data)" }
$sellerId = $createSeller.Data.seller.id
Write-Host "Seller created successfully with ID: $sellerId"

Write-Host "3. Listing Sellers..."
$listSellers = Invoke-Api -Method "GET" -Endpoint "/api/admin/sellers" -Token $adminToken
if ($listSellers.Status -ne 200) { throw "List sellers failed: $($listSellers.Status)" }
Write-Host "Found $($listSellers.Data.totalCount) sellers."

Write-Host "4. Updating Seller Status..."
$updateStatus = Invoke-Api -Method "PATCH" -Endpoint "/api/admin/sellers/$sellerId/status" -Body @{ status = "active" } -Token $adminToken
if ($updateStatus.Status -ne 204) { throw "Update status failed: $($updateStatus.Status)" }
Write-Host "Seller status updated successfully."

Write-Host "5. Logging in as Seller Owner..."
$sellerLogin = Invoke-Api -Method "POST" -Endpoint "/api/auth/login" -Body @{ email = "owner@super.com"; password = "storePassword123" }
if ($sellerLogin.Status -ne 200) { throw "Seller login failed: $($sellerLogin.Status)" }
$sellerToken = $sellerLogin.Data.accessToken
Write-Host "Seller logged in successfully."

Write-Host "6. Getting Seller Profile..."
$sellerMe = Invoke-Api -Method "GET" -Endpoint "/api/seller/me" -Token $sellerToken
if ($sellerMe.Status -ne 200) { throw "Get seller profile failed: $($sellerMe.Status)" }
Write-Host "Seller profile retrieved: $($sellerMe.Data.seller.brandName)"

Write-Host "7. Verifying Seller cannot access Admin endpoints..."
$adminAccess = Invoke-Api -Method "GET" -Endpoint "/api/admin/sellers" -Token $sellerToken
if ($adminAccess.Status -ne 403 -and $adminAccess.Status -ne 401) { throw "Seller accessed admin endpoint! Status: $($adminAccess.Status)" }
Write-Host "Seller blocked from admin endpoint (Status: $($adminAccess.Status))."

Write-Host "8. Creating Customer and verifying Customer cannot access Seller endpoints..."
$customerReq = @{ name = "Test Customer"; email = "cust@test.com"; password = "custPassword123" }
$custReg = Invoke-Api -Method "POST" -Endpoint "/api/auth/register" -Body $customerReq
if ($custReg.Status -ne 201) { throw "Customer registration failed: $($custReg.Status)" }
$customerLogin = Invoke-Api -Method "POST" -Endpoint "/api/auth/login" -Body @{ email = "cust@test.com"; password = "custPassword123" }
$custToken = $customerLogin.Data.accessToken

$custAccess = Invoke-Api -Method "GET" -Endpoint "/api/seller/me" -Token $custToken
if ($custAccess.Status -ne 403 -and $custAccess.Status -ne 401) { throw "Customer accessed seller endpoint! Status: $($custAccess.Status)" }
Write-Host "Customer blocked from seller endpoint (Status: $($custAccess.Status))."

Write-Host "All E2E tests passed successfully!"
