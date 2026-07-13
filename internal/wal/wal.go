package wal

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync"

	"flamingodb/internal/storage/page"
)

// RecordType represents the action logged in the WAL.
type RecordType byte

const (
	Begin RecordType = iota
	Update
	Commit
	Abort
)

// LogRecord represents a single entry in the Write-Ahead Log.
type LogRecord struct {
	LSN    uint64
	TxID   uint64
	Type   RecordType
	PageID page.PageID
	Data   []byte // Page content (after-image)
}

// WAL manages appending and reading log records from the log file.
type WAL struct {
	file     *os.File
	filename string
	mu       sync.Mutex
	lsn      uint64
}

// Open opens or creates a WAL file.
func Open(filename string) (*WAL, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return &WAL{
		file:     file,
		filename: filename,
	}, nil
}

// Close closes the WAL file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

// Filename returns the path of the WAL file.
func (w *WAL) Filename() string {
	return w.filename
}

// Append logs a new record to the WAL and returns its Log Sequence Number (LSN).
func (w *WAL) Append(txID uint64, recType RecordType, pageID page.PageID, data []byte) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.lsn++
	rec := &LogRecord{
		LSN:    w.lsn,
		TxID:   txID,
		Type:   recType,
		PageID: pageID,
		Data:   data,
	}

	bytes, err := SerializeRecord(rec)
	if err != nil {
		return 0, err
	}

	_, err = w.file.Write(bytes)
	if err != nil {
		return 0, err
	}

	return w.lsn, nil
}

// Sync flushes the WAL changes to physical disk (fsync).
func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Sync()
}

// Truncate clears all records from the WAL file and resets the LSN sequence.
func (w *WAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.file.Truncate(0); err != nil {
		return err
	}
	if _, err := w.file.Seek(0, 0); err != nil {
		return err
	}
	w.lsn = 0
	return nil
}

// ReadAllRecords reads all records from the beginning of the WAL.
func (w *WAL) ReadAllRecords() ([]*LogRecord, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	_, err := w.file.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	var records []*LogRecord
	for {
		rec, err := DeserializeRecord(w.file)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, err
		}
		records = append(records, rec)
	}

	return records, nil
}

// SerializeRecord encodes a LogRecord into a binary byte slice.
func SerializeRecord(rec *LogRecord) ([]byte, error) {
	size := 10
	if rec.Type == Update {
		size += 8 + len(rec.Data)
	}
	size += 4 // checksum

	buf := make([]byte, size)
	buf[0] = 0xFD
	buf[1] = byte(rec.Type)
	binary.LittleEndian.PutUint64(buf[2:10], rec.TxID)

	offset := 10
	if rec.Type == Update {
		binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(rec.PageID))
		binary.LittleEndian.PutUint32(buf[offset+4:offset+8], uint32(len(rec.Data)))
		copy(buf[offset+8:offset+8+len(rec.Data)], rec.Data)
		offset += 8 + len(rec.Data)
	}

	checksum := crc32.ChecksumIEEE(buf[:offset])
	binary.LittleEndian.PutUint32(buf[offset:offset+4], checksum)

	return buf, nil
}

// DeserializeRecord decodes a LogRecord from an io.Reader and verifies its CRC32 checksum.
func DeserializeRecord(r io.Reader) (*LogRecord, error) {
	header := make([]byte, 10)
	_, err := io.ReadFull(r, header)
	if err != nil {
		return nil, err
	}

	if header[0] != 0xFD {
		return nil, fmt.Errorf("invalid magic byte in WAL record: %x", header[0])
	}

	recType := RecordType(header[1])
	txID := binary.LittleEndian.Uint64(header[2:10])

	rec := &LogRecord{
		TxID: txID,
		Type: recType,
	}

	var dataBuf []byte
	var pageID uint32
	if recType == Update {
		meta := make([]byte, 8)
		_, err := io.ReadFull(r, meta)
		if err != nil {
			return nil, err
		}
		pageID = binary.LittleEndian.Uint32(meta[0:4])
		dataLen := binary.LittleEndian.Uint32(meta[4:8])

		dataBuf = make([]byte, dataLen)
		_, err = io.ReadFull(r, dataBuf)
		if err != nil {
			return nil, err
		}

		rec.PageID = page.PageID(pageID)
		rec.Data = dataBuf
	}

	checksumBuf := make([]byte, 4)
	_, err = io.ReadFull(r, checksumBuf)
	if err != nil {
		return nil, err
	}
	expectedChecksum := binary.LittleEndian.Uint32(checksumBuf)

	// Verify checksum
	bufSize := 10
	if recType == Update {
		bufSize += 8 + len(dataBuf)
	}
	chkBuf := make([]byte, bufSize)
	copy(chkBuf[:10], header)
	if recType == Update {
		binary.LittleEndian.PutUint32(chkBuf[10:14], pageID)
		binary.LittleEndian.PutUint32(chkBuf[14:18], uint32(len(dataBuf)))
		copy(chkBuf[18:], dataBuf)
	}

	actualChecksum := crc32.ChecksumIEEE(chkBuf)
	if actualChecksum != expectedChecksum {
		return nil, fmt.Errorf("checksum mismatch in WAL record: expected %d, got %d", expectedChecksum, actualChecksum)
	}

	return rec, nil
}
