# Frontend Polish Audit (UI-POLISH-1)

| app | page | problem | fix | file | status | notes |
|---|---|---|---|---|---|---|

## api-client
| app | page | problem | fix | file | status | notes |
|---|---|---|---|---|---|---|
| api-client | all | Unsafe arrays | defaulted arrays to [] | public.ts, admin.ts, seller.ts, customer.ts | fixed | |

## shop
| app | page | problem | fix | file | status | notes |
|---|---|---|---|---|---|---|
| shop | Cart | Used mock recommendations | Replaced with public catalog fetch | Cart.tsx | fixed | Removed mock items |
| shop | ProductDetail | Hardcoded mock recommendations | Replaced with getPublicProducts() call | ProductDetail.tsx | fixed | Added safety bounds |
| shop | ProductCard | Missing image errors | Added generic gray fallback | ProductCard.tsx | fixed | Handled missing mainImageUrl |
| shop | NewArrivals | Technical mock text visible | Cleaned up description | NewArrivals.tsx | fixed | |

## seller
| app | page | problem | fix | file | status | notes |
|---|---|---|---|---|---|---|
| seller | SellerProducts | Unused Copy import | Removed import | SellerProducts.tsx | fixed | Build failing otherwise |
| seller | SellerProducts | Uses `alert()` for errors | Replaced with `setError` state | SellerProducts.tsx | fixed | |
| seller | SellerProductEdit | Uses `alert()` for file validation | Replaced with `setError` state | SellerProductEdit.tsx | fixed | |
| seller | SellerProductNew | Uses `alert()` for file validation | Replaced with `setError` state | SellerProductNew.tsx | fixed | |

## admin
| app | page | problem | fix | file | status | notes |
|---|---|---|---|---|---|---|
| admin | AdminAuctionCreate | Uses `alert()` for success/fail | Replaced with inline state | AdminAuctionCreate.tsx | fixed | |
| admin | AdminAuctionDetail | Uses `alert()` for success/fail | Replaced with inline state | AdminAuctionDetail.tsx | fixed | |
| admin | AdminAuctionsList | Uses `alert()` for success/fail | Replaced with inline state | AdminAuctionsList.tsx | fixed | |
| admin | AdminBrands | Uses `alert()` for success/fail | Replaced with inline state | AdminBrands.tsx | fixed | |
| admin | AdminCatalog | Uses `alert()` for success/fail | Replaced with inline state | AdminCatalog.tsx | fixed | |
| admin | AdminSellers | Uses `alert()` for success/fail | Replaced with inline state | AdminSellers.tsx | fixed | Fixed mojibake |
| admin | AdminStaff | Uses `alert()` for success/fail | Replaced with inline state | AdminStaff.tsx | fixed | |

## known gaps
*   `AdminProducts` and `AdminAuctions` might still be slightly unpolished, but they fulfill the minimum UX requirements without crashing.

