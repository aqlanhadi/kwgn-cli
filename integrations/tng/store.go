package tng

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// The store is one JSON array per source in a directory. Each sync merges into
// it by the source's own transaction key, so history the portal later drops
// (the eWallet keeps about a year) stays on disk.

// rowKey returns the identity of a row and the timestamp it sorts by.
func rowKey(src Source, raw json.RawMessage) (key, when string, err error) {
	switch src {
	case SourceEwallet:
		var r struct {
			WalletRef string `json:"walletRef"`
			TxnID     string `json:"txnId"`
			TxnDate   string `json:"txnDate"`
			TxnAmount string `json:"txnAmount"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return "", "", err
		}
		key = r.WalletRef
		if key == "" {
			key = r.TxnID + "|" + r.TxnDate + "|" + r.TxnAmount
		}
		return key, r.TxnDate, nil
	case SourceCard:
		// TNG's own server lists a few card rows twice; this key folds them.
		var r struct {
			CardMfgNo  string `json:"cardMfgNo"`
			TxSeqNo    string `json:"txSeqNo"`
			TxID       string `json:"txId"`
			TxDatetime string `json:"txDatetime"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return "", "", err
		}
		return r.CardMfgNo + "|" + r.TxSeqNo + "|" + r.TxID, r.TxDatetime, nil
	case SourceRFID:
		var r struct {
			ID                  string `json:"id"`
			TransactionDateTime string `json:"transactionDateTime"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return "", "", err
		}
		return r.ID, r.TransactionDateTime, nil
	}
	return "", "", fmt.Errorf("tng: unknown source %q", src)
}

// MergeResult counts what a merge changed.
type MergeResult struct {
	Added, Updated, Total int
}

// Merge folds incoming rows into existing ones. A row already present is
// replaced by the incoming copy, since the portal may have revised it.
func Merge(src Source, existing, incoming Rows) (Rows, MergeResult, error) {
	type item struct {
		when string
		raw  json.RawMessage
	}
	byKey := map[string]item{}
	var res MergeResult
	for _, raw := range existing {
		k, w, err := rowKey(src, raw)
		if err != nil {
			return nil, res, err
		}
		byKey[k] = item{w, raw}
	}
	for _, raw := range incoming {
		k, w, err := rowKey(src, raw)
		if err != nil {
			return nil, res, err
		}
		old, ok := byKey[k]
		switch {
		case !ok:
			res.Added++
		case !bytes.Equal(compact(old.raw), compact(raw)):
			res.Updated++
		}
		byKey[k] = item{w, raw}
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := byKey[keys[i]], byKey[keys[j]]
		if a.when != b.when {
			return a.when < b.when
		}
		return keys[i] < keys[j]
	})
	out := make(Rows, len(keys))
	for i, k := range keys {
		out[i] = byKey[k].raw
	}
	res.Total = len(out)
	return out, res, nil
}

func compact(raw json.RawMessage) []byte {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return raw
	}
	return b.Bytes()
}

func storePath(dir string, src Source) string {
	return filepath.Join(dir, string(src)+".json")
}

// LoadRows reads a source's file; a missing file is an empty history.
func LoadRows(dir string, src Source) (Rows, error) {
	b, err := os.ReadFile(storePath(dir, src))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rows Rows
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("tng: reading %s: %w", storePath(dir, src), err)
	}
	return rows, nil
}

// SaveRows writes a source's file atomically, one row per line.
func SaveRows(dir string, src Source, rows Rows) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	var b bytes.Buffer
	b.WriteString("[\n")
	for i, r := range rows {
		b.Write(compact(r))
		if i < len(rows)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("]\n")
	path := storePath(dir, src)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b.Bytes(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
