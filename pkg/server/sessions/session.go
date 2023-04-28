/*
Copyright 2022 Codenotary Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sessions

import (
	"context"
	"sync"
	"time"

	"github.com/codenotary/immudb/embedded/document"
	"github.com/codenotary/immudb/embedded/sql"

	"github.com/codenotary/immudb/embedded/multierr"
	"github.com/codenotary/immudb/pkg/auth"
	"github.com/codenotary/immudb/pkg/database"
	"github.com/codenotary/immudb/pkg/errors"
	"github.com/codenotary/immudb/pkg/logger"
	"github.com/codenotary/immudb/pkg/server/sessions/internal/transactions"
	"google.golang.org/grpc/metadata"
)

var (
	ErrPaginatedDocumentReaderNotFound = errors.New("paginated document reader not found")
)

type PaginatedDocumentReader struct {
	LastPageNumber uint32                  // last read page number
	LastPageSize   uint32                  // number of items per page
	TotalRead      uint32                  // total number of items read
	Reader         document.DocumentReader // reader to read from
}

type Session struct {
	mux                      sync.RWMutex
	id                       string
	user                     *auth.User
	database                 database.DB
	creationTime             time.Time
	lastActivityTime         time.Time
	transactions             map[string]transactions.Transaction
	paginatedDocumentReaders map[string]*PaginatedDocumentReader // map from query names to sql.RowReader objects
	log                      logger.Logger
}

func NewSession(sessionID string, user *auth.User, db database.DB, log logger.Logger) *Session {
	now := time.Now()
	return &Session{
		id:                       sessionID,
		user:                     user,
		database:                 db,
		creationTime:             now,
		lastActivityTime:         now,
		transactions:             make(map[string]transactions.Transaction),
		log:                      log,
		paginatedDocumentReaders: make(map[string]*PaginatedDocumentReader),
	}
}

func (s *Session) NewTransaction(ctx context.Context, opts *sql.TxOptions) (transactions.Transaction, error) {
	s.mux.Lock()
	defer s.mux.Unlock()

	tx, err := transactions.NewTransaction(ctx, opts, s.database, s.id)
	if err != nil {
		return nil, err
	}

	s.transactions[tx.GetID()] = tx
	return tx, nil
}

func (s *Session) RemoveTransaction(transactionID string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.removeTransaction(transactionID)
}

// not thread safe
func (s *Session) removeTransaction(transactionID string) error {
	if _, ok := s.transactions[transactionID]; ok {
		delete(s.transactions, transactionID)
		return nil
	}
	return ErrTransactionNotFound
}

func (s *Session) ClosePaginatedDocumentReaders() error {
	s.mux.Lock()
	defer s.mux.Unlock()

	merr := multierr.NewMultiErr()

	for qname := range s.paginatedDocumentReaders {
		if err := s.DeletePaginatedDocumentReader(qname); err != nil {
			s.log.Errorf("Error while removing paginated reader: %v", err)
			merr.Append(err)
		}
	}

	return merr.Reduce()
}

func (s *Session) RollbackTransactions() error {
	s.mux.Lock()
	defer s.mux.Unlock()

	merr := multierr.NewMultiErr()

	for _, tx := range s.transactions {
		s.log.Debugf("Deleting transaction %s", tx.GetID())

		if err := tx.Rollback(); err != nil {
			s.log.Errorf("Error while rolling back transaction %s: %v", tx.GetID(), err)
			merr.Append(err)
			continue
		}

		if err := s.removeTransaction(tx.GetID()); err != nil {
			s.log.Errorf("Error while removing transaction %s: %v", tx.GetID(), err)
			merr.Append(err)
			continue
		}
	}

	return merr.Reduce()
}

func (s *Session) GetID() string {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.id
}

func (s *Session) GetTransaction(transactionID string) (transactions.Transaction, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()

	tx, ok := s.transactions[transactionID]
	if !ok {
		return nil, transactions.ErrTransactionNotFound
	}

	return tx, nil
}

func GetSessionIDFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", ErrNoSessionAuthDataProvided
	}
	authHeader, ok := md["sessionid"]
	if !ok || len(authHeader) < 1 {
		return "", ErrNoSessionAuthDataProvided
	}
	sessionID := authHeader[0]
	if sessionID == "" {
		return "", ErrNoSessionIDPresent
	}
	return sessionID, nil
}

func GetTransactionIDFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", ErrNoTransactionAuthDataProvided
	}
	authHeader, ok := md["transactionid"]
	if !ok || len(authHeader) < 1 {
		return "", ErrNoTransactionAuthDataProvided
	}
	transactionID := authHeader[0]
	if transactionID == "" {
		return "", ErrNoTransactionIDPresent
	}
	return transactionID, nil
}

func (s *Session) GetUser() *auth.User {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.user
}

func (s *Session) GetDatabase() database.DB {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.database
}

func (s *Session) SetDatabase(db database.DB) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.database = db
}

func (s *Session) GetLastActivityTime() time.Time {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.lastActivityTime
}

func (s *Session) SetLastActivityTime(t time.Time) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.lastActivityTime = t
}

func (s *Session) GetCreationTime() time.Time {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.creationTime
}

func (s *Session) SetPaginatedDocumentReader(queryName string, reader *PaginatedDocumentReader) {
	s.mux.Lock()
	defer s.mux.Unlock()

	// add the reader to the paginatedDocumentReaders map
	s.paginatedDocumentReaders[queryName] = reader
}

func (s *Session) GetPaginatedDocumentReader(queryName string) (*PaginatedDocumentReader, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()

	// get the io.Reader object for the specified query name
	reader, ok := s.paginatedDocumentReaders[queryName]
	if !ok {
		return nil, ErrPaginatedDocumentReaderNotFound
	}

	return reader, nil
}

func (s *Session) DeletePaginatedDocumentReader(queryName string) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	// get the io.Reader object for the specified query name
	reader, ok := s.paginatedDocumentReaders[queryName]
	if !ok {
		return ErrPaginatedDocumentReaderNotFound
	}

	// close the reader
	err := reader.Reader.Close()

	delete(s.paginatedDocumentReaders, queryName)

	if err != nil {
		return err
	}

	return nil
}

func (s *Session) UpdatePaginatedDocumentReader(queryName string, lastPage uint32, lastPageSize uint32, totalDocsRead int) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	// get the io.Reader object for the specified query name
	reader, ok := s.paginatedDocumentReaders[queryName]
	if !ok {
		return ErrPaginatedDocumentReaderNotFound
	}

	if lastPage > 0 {
		reader.LastPageNumber = (lastPage)
	}
	if lastPageSize > 0 {
		reader.LastPageSize = (lastPageSize)
	}
	if totalDocsRead > 0 {
		reader.TotalRead = uint32(totalDocsRead)
	}

	return nil
}

func (s *Session) GetPaginatedDocumentReadersCount() int {
	return len(s.paginatedDocumentReaders)
}
