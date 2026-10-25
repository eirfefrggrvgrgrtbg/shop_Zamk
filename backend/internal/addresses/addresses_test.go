package addresses_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/addresses"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// SHOP.ADDRESSES.1 on a disposable database: the address book through the real handlers.

type kit struct {
	t   *testing.T
	pg  *postgres.Client
	h   *addresses.Handler
	a   uuid.UUID
	b   uuid.UUID
	ctx context.Context
}

func newKit(t *testing.T) *kit {
	t.Helper()
	ctx := context.Background()
	pg, err := postgres.NewClient(ctx, testutil.DisposableDatabaseURL(t))
	require.NoError(t, err)
	t.Cleanup(pg.Close)
	testutil.MustBeDisposableDatabase(t, pg.Pool)
	k := &kit{t: t, pg: pg, ctx: ctx, a: uuid.New(), b: uuid.New()}
	for i, id := range []uuid.UUID{k.a, k.b} {
		_, err := pg.Pool.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, role) VALUES ($1,$2,$3,'h','customer')`, id, fmt.Sprintf("Покупатель %d", i), id.String()[:8]+"@addr.test")
		require.NoError(t, err)
	}
	k.h = addresses.NewHandler(addresses.NewService(addresses.NewRepository(pg)))
	return k
}

func valid() map[string]any {
	return map[string]any{"label": "Дом", "recipientName": "Анна Иванова", "phone": "+7 999 000-11-22", "city": "Москва", "street": "Тверская", "house": "1", "apartment": "12", "postalCode": "101000"}
}

type res struct {
	Code  int
	Body  []byte
	Error struct{ Code, Message, Field string }
}

func (k *kit) call(user *uuid.UUID, method, path string, params map[string]string, body any) res {
	k.t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	rctx := chi.NewRouteContext()
	for key, v := range params {
		rctx.URLParams.Add(key, v)
	}
	c := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	if user != nil {
		c = context.WithValue(c, "userID", *user)
	}
	w := httptest.NewRecorder()
	switch {
	case method == http.MethodGet:
		k.h.ListAddresses(w, req.WithContext(c))
	case method == http.MethodPost && strings.HasSuffix(path, "/default"):
		k.h.SetDefault(w, req.WithContext(c))
	case method == http.MethodPost:
		k.h.CreateAddress(w, req.WithContext(c))
	case method == http.MethodPatch:
		k.h.UpdateAddress(w, req.WithContext(c))
	case method == http.MethodDelete:
		k.h.DeleteAddress(w, req.WithContext(c))
	}
	out := res{Code: w.Code, Body: w.Body.Bytes()}
	var env struct {
		Error struct{ Code, Message, Field string } `json:"error"`
	}
	_ = json.Unmarshal(out.Body, &env)
	out.Error = env.Error
	return out
}

func (k *kit) create(user uuid.UUID, body map[string]any) addresses.Address {
	k.t.Helper()
	r := k.call(&user, http.MethodPost, "/addresses", nil, body)
	require.Equal(k.t, http.StatusCreated, r.Code, string(r.Body))
	var a addresses.Address
	require.NoError(k.t, json.Unmarshal(r.Body, &a))
	return a
}

func (k *kit) list(user uuid.UUID) []addresses.Address {
	k.t.Helper()
	r := k.call(&user, http.MethodGet, "/addresses", nil, nil)
	require.Equal(k.t, http.StatusOK, r.Code)
	var out []addresses.Address
	require.NoError(k.t, json.Unmarshal(r.Body, &out))
	return out
}

func (k *kit) defaults(user uuid.UUID) int {
	n := 0
	for _, a := range k.list(user) {
		if a.IsDefault {
			n++
		}
	}
	return n
}

func TestAddressBook(t *testing.T) {
	k := newKit(t)

	t.Run("create: trimmed, real timestamps, first address becomes the main one, empty list is an array", func(t *testing.T) {
		empty := k.call(&k.a, http.MethodGet, "/addresses", nil, nil)
		assert.JSONEq(t, `[]`, string(empty.Body))
		body := valid()
		body["recipientName"] = "  Анна Иванова  "
		a := k.create(k.a, body)
		assert.Equal(t, "Анна Иванова", a.RecipientName)
		assert.True(t, a.IsDefault, "the first address is the main one")
		assert.True(t, a.CreatedAt.After(time.Now().Add(-time.Minute)), "created_at is real: %s", a.CreatedAt)
		assert.True(t, a.UpdatedAt.After(time.Now().Add(-time.Minute)))
	})

	t.Run("a new main address replaces the old one: exactly one main address", func(t *testing.T) {
		body := valid()
		body["label"] = "Офис"
		body["isDefault"] = true
		k.create(k.a, body)
		assert.Equal(t, 1, k.defaults(k.a))
		list := k.list(k.a)
		assert.Equal(t, "Офис", *list[0].Label, "newest first, and it is the main one")
		assert.True(t, list[0].IsDefault)
	})

	t.Run("validation: required, length, control characters, phone, postal code", func(t *testing.T) {
		cases := []struct {
			field string
			value any
		}{
			{"recipientName", "   "}, {"phone", ""}, {"city", ""}, {"street", " "}, {"house", ""},
			{"city", strings.Repeat("я", 121)}, {"comment", strings.Repeat("x", 501)}, {"label", strings.Repeat("x", 61)},
			{"street", "Тверская\x00"}, {"phone", "abc"}, {"phone", "12345"}, {"phone", "+7 (999) 000-11-22 ext 5"},
			{"postalCode", "10<1>"},
		}
		before := len(k.list(k.a))
		for _, c := range cases {
			body := valid()
			body[c.field] = c.value
			r := k.call(&k.a, http.MethodPost, "/addresses", nil, body)
			require.Equal(t, http.StatusBadRequest, r.Code, "%s=%v %s", c.field, c.value, r.Body)
			assert.Equal(t, "validation_error", r.Error.Code)
			assert.Equal(t, c.field, r.Error.Field)
			assert.Regexp(t, `[А-Яа-я]`, r.Error.Message, "message is in Russian")
		}
		assert.Equal(t, before, len(k.list(k.a)), "nothing was saved")
		oversized := valid()
		oversized["comment"] = strings.Repeat("x", 20000)
		assert.Equal(t, http.StatusBadRequest, k.call(&k.a, http.MethodPost, "/addresses", nil, oversized).Code, "oversized body")
	})

	t.Run("update: partial patch keeps the rest; an empty required field is refused; unknown id is 404", func(t *testing.T) {
		a := k.list(k.a)[0]
		id := map[string]string{"id": a.ID.String()}
		r := k.call(&k.a, http.MethodPatch, "/addresses/x", id, map[string]any{"apartment": "99", "house": " 7 "})
		require.Equal(t, http.StatusOK, r.Code, string(r.Body))
		var u addresses.Address
		require.NoError(t, json.Unmarshal(r.Body, &u))
		assert.Equal(t, "7", u.House)
		assert.Equal(t, "99", *u.Apartment)
		assert.Equal(t, a.City, u.City, "untouched fields stay")
		assert.Equal(t, a.IsDefault, u.IsDefault)
		bad := k.call(&k.a, http.MethodPatch, "/addresses/x", id, map[string]any{"city": ""})
		assert.Equal(t, http.StatusBadRequest, bad.Code)
		assert.Equal(t, "city", bad.Error.Field)
		assert.Equal(t, "7", k.list(k.a)[0].House, "the refused patch changed nothing")
		missing := k.call(&k.a, http.MethodPatch, "/addresses/x", map[string]string{"id": uuid.NewString()}, map[string]any{"city": "Казань"})
		assert.Equal(t, http.StatusNotFound, missing.Code)
	})

	t.Run("set main: switches the main address, never two", func(t *testing.T) {
		list := k.list(k.a)
		var other addresses.Address
		for _, a := range list {
			if !a.IsDefault {
				other = a
			}
		}
		r := k.call(&k.a, http.MethodPost, "/addresses/x/default", map[string]string{"id": other.ID.String()}, nil)
		require.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, 1, k.defaults(k.a))
		for _, a := range k.list(k.a) {
			assert.Equal(t, a.ID == other.ID, a.IsDefault)
		}
	})

	t.Run("delete the main address: the newest remaining one becomes main", func(t *testing.T) {
		kk := newKit(t)
		first := kk.create(kk.a, valid())
		second := kk.create(kk.a, valid())
		assert.True(t, first.IsDefault)
		assert.False(t, second.IsDefault)
		r := kk.call(&kk.a, http.MethodDelete, "/addresses/x", map[string]string{"id": first.ID.String()}, nil)
		require.Equal(t, http.StatusOK, r.Code)
		list := kk.list(kk.a)
		require.Len(t, list, 1)
		assert.Equal(t, second.ID, list[0].ID)
		assert.True(t, list[0].IsDefault)
	})

	t.Run("limit: 20 addresses per customer, then a clear 409", func(t *testing.T) {
		kk := newKit(t)
		for i := 0; i < 20; i++ {
			kk.create(kk.a, valid())
		}
		r := kk.call(&kk.a, http.MethodPost, "/addresses", nil, valid())
		assert.Equal(t, http.StatusConflict, r.Code)
		assert.Equal(t, "address_limit", r.Error.Code)
		assert.Len(t, kk.list(kk.a), 20)
		assert.Equal(t, 1, kk.defaults(kk.a))
	})
}

func TestAddressBookIsolation(t *testing.T) {
	k := newKit(t)
	a := k.create(k.a, valid())
	id := map[string]string{"id": a.ID.String()}

	t.Run("another customer cannot see, change, make main or delete it", func(t *testing.T) {
		assert.Empty(t, k.list(k.b), "B's list holds nothing of A's")
		assert.Equal(t, http.StatusNotFound, k.call(&k.b, http.MethodPatch, "/addresses/x", id, map[string]any{"city": "Взлом"}).Code)
		assert.Equal(t, http.StatusNotFound, k.call(&k.b, http.MethodPost, "/addresses/x/default", id, nil).Code)
		del := k.call(&k.b, http.MethodDelete, "/addresses/x", id, nil)
		assert.Equal(t, http.StatusOK, del.Code, "deleting someone else's id is a no-op, not a data leak")
		mine := k.list(k.a)
		require.Len(t, mine, 1)
		assert.Equal(t, "Москва", mine[0].City, "A's address is unchanged and still there")
		assert.Equal(t, http.StatusNotFound, k.call(&k.b, http.MethodPatch, "/addresses/x", map[string]string{"id": uuid.NewString()}, map[string]any{"city": "x"}).Code, "foreign and missing look the same")
	})

	t.Run("no session: 401 everywhere and nothing is read or written", func(t *testing.T) {
		for _, m := range []struct{ method, path string }{{http.MethodGet, "/addresses"}, {http.MethodPost, "/addresses"}, {http.MethodPatch, "/addresses/x"}, {http.MethodDelete, "/addresses/x"}, {http.MethodPost, "/addresses/x/default"}} {
			r := k.call(nil, m.method, m.path, id, valid())
			assert.Equal(t, http.StatusUnauthorized, r.Code, m.method+" "+m.path)
			assert.NotContains(t, string(r.Body), "Москва")
		}
		assert.Len(t, k.list(k.a), 1)
	})

	t.Run("a bad id is 400, not a server error", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, k.call(&k.a, http.MethodDelete, "/addresses/x", map[string]string{"id": "nope"}, nil).Code)
	})
}

// SHOP.DELIVERY.DETAILS.1: parallel requests queue up on the customer's lock instead of colliding.
func TestAddressBookConcurrency(t *testing.T) {
	run := func(k *kit, n int, body func(i int) map[string]any) (codes []int) {
		var wg sync.WaitGroup
		out := make([]int, n)
		start := make(chan struct{})
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				out[i] = k.call(&k.a, http.MethodPost, "/addresses", nil, body(i)).Code
			}(i)
		}
		close(start)
		wg.Wait()
		return out
	}

	t.Run("two first addresses at once: both saved, exactly one main, no 500", func(t *testing.T) {
		for round := 0; round < 5; round++ { // the race is timing-dependent: repeat on fresh books
			k := newKit(t)
			codes := run(k, 2, func(i int) map[string]any { b := valid(); b["label"] = fmt.Sprintf("Адрес %d", i); return b })
			for _, c := range codes {
				require.Equal(t, http.StatusCreated, c, "round %d: %v", round, codes)
			}
			require.Len(t, k.list(k.a), 2)
			require.Equal(t, 1, k.defaults(k.a), "round %d", round)
		}
	})

	t.Run("two first addresses, both asking to be main: both saved, exactly one main", func(t *testing.T) {
		for round := 0; round < 3; round++ {
			k := newKit(t)
			codes := run(k, 2, func(i int) map[string]any { b := valid(); b["isDefault"] = true; return b })
			for _, c := range codes {
				require.Equal(t, http.StatusCreated, c, "%v", codes)
			}
			require.Len(t, k.list(k.a), 2)
			require.Equal(t, 1, k.defaults(k.a))
		}
	})

	t.Run("the limit holds under concurrency: 19 saved, 6 more at once, exactly one gets in", func(t *testing.T) {
		k := newKit(t)
		for i := 0; i < 19; i++ {
			k.create(k.a, valid())
		}
		codes := run(k, 6, func(int) map[string]any { return valid() })
		created, limited := 0, 0
		for _, c := range codes {
			switch c {
			case http.StatusCreated:
				created++
			case http.StatusConflict:
				limited++
			default:
				t.Fatalf("unexpected status %d in %v", c, codes)
			}
		}
		require.Equal(t, 1, created, "%v", codes)
		require.Equal(t, 5, limited)
		require.Len(t, k.list(k.a), 20)
		require.Equal(t, 1, k.defaults(k.a))
	})
}
