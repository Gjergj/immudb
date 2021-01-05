package database

import (
	"strconv"
	"testing"

	"github.com/codenotary/immudb/embedded/store"
	"github.com/codenotary/immudb/pkg/api/schema"
	"github.com/stretchr/testify/require"
)

func TestSetBatch(t *testing.T) {
	db, closer := makeDb()
	defer closer()

	batchSize := 100

	for b := 0; b < 10; b++ {
		kvList := make([]*schema.KeyValue, batchSize)

		for i := 0; i < batchSize; i++ {
			key := []byte(strconv.FormatUint(uint64(i), 10))
			value := []byte(strconv.FormatUint(uint64(b*batchSize+batchSize+i), 10))
			kvList[i] = &schema.KeyValue{
				Key:   key,
				Value: value,
			}
		}

		md, err := db.Set(&schema.SetRequest{KVs: kvList})
		require.NoError(t, err)
		require.Equal(t, uint64(b+1), md.Id)

		for i := 0; i < batchSize; i++ {
			key := []byte(strconv.FormatUint(uint64(i), 10))
			value := []byte(strconv.FormatUint(uint64(b*batchSize+batchSize+i), 10))
			entry, err := db.Get(&schema.KeyRequest{Key: key, SinceTx: md.Id})
			require.NoError(t, err)
			require.Equal(t, value, entry.Value)
			require.Equal(t, uint64(b+1), entry.Tx)

			vitem, err := db.VerifiableGet(&schema.VerifiableGetRequest{KeyRequest: &schema.KeyRequest{Key: key}}) //no prev root
			require.NoError(t, err)
			require.Equal(t, key, vitem.Entry.Key)
			require.Equal(t, value, vitem.Entry.Value)
			require.Equal(t, entry.Tx, vitem.Entry.Tx)

			tx := schema.TxFrom(vitem.VerifiableTx.Tx)

			inclusionProof := schema.InclusionProofFrom(vitem.InclusionProof)
			verifies := store.VerifyInclusion(
				inclusionProof,
				EncodeKV(vitem.Entry.Key, vitem.Entry.Value),
				tx.Eh(),
			)
			require.True(t, verifies)
		}
	}
}

func TestSetBatchInvalidKvKey(t *testing.T) {
	db, closer := makeDb()
	defer closer()

	_, err := db.Set(&schema.SetRequest{
		KVs: []*schema.KeyValue{
			{
				Key:   []byte{},
				Value: []byte(`val`),
			},
		}})
	require.Equal(t, store.ErrIllegalArguments, err)
}

func TestSetBatchDuplicatedKey(t *testing.T) {
	db, closer := makeDb()
	defer closer()

	_, err := db.Set(&schema.SetRequest{
		KVs: []*schema.KeyValue{
			{
				Key:   []byte(`key`),
				Value: []byte(`val`),
			},
			{
				Key:   []byte(`key`),
				Value: []byte(`val`),
			},
		}},
	)
	require.Equal(t, store.ErrDuplicatedKey, err)
}

func TestExecAllOps(t *testing.T) {
	db, closer := makeDb()
	defer closer()

	batchSize := 100

	for b := 0; b < 10; b++ {
		atomicOps := make([]*schema.Op, batchSize*2)

		for i := 0; i < batchSize; i++ {
			key := []byte(strconv.FormatUint(uint64(i), 10))
			value := []byte(strconv.FormatUint(uint64(b*batchSize+batchSize+i), 10))
			atomicOps[i] = &schema.Op{
				Operation: &schema.Op_Kv{
					Kv: &schema.KeyValue{
						Key:   key,
						Value: value,
					},
				},
			}
		}

		for i := 0; i < batchSize; i++ {

			atomicOps[i+batchSize] = &schema.Op{
				Operation: &schema.Op_ZAdd{
					ZAdd: &schema.ZAddRequest{
						Set:   []byte(`mySet`),
						Score: 0.6,
						Key:   atomicOps[i].Operation.(*schema.Op_Kv).Kv.Key,
						AtTx:  0,
					},
				},
			}
		}

		idx, err := db.ExecAll(&schema.ExecAllRequest{Operations: atomicOps})
		require.NoError(t, err)
		require.Equal(t, uint64(b+1), idx.Id)
	}

	zScanOpt := &schema.ZScanRequest{
		Set:     []byte(`mySet`),
		SinceTx: 10,
	}
	zList, err := db.ZScan(zScanOpt)
	require.NoError(t, err)
	println(len(zList.Entries))
	require.Len(t, zList.Entries, batchSize)
}

func TestExecAllOpsZAddOnMixedAlreadyPersitedNotPersistedItems(t *testing.T) {
	db, closer := makeDb()
	defer closer()

	idx, _ := db.Set(&schema.SetRequest{
		KVs: []*schema.KeyValue{
			{Key: []byte(`persistedKey`),
				Value: []byte(`persistedVal`),
			},
		},
	})

	// Ops payload
	aOps := &schema.ExecAllRequest{
		Operations: []*schema.Op{
			{
				Operation: &schema.Op_Kv{
					Kv: &schema.KeyValue{
						Key:   []byte(`notPersistedKey`),
						Value: []byte(`notPersistedVal`),
					},
				},
			},
			{
				Operation: &schema.Op_ZAdd{
					ZAdd: &schema.ZAddRequest{
						Set:   []byte(`mySet`),
						Score: 0.6,
						Key:   []byte(`notPersistedKey`)},
				},
			},
			{
				Operation: &schema.Op_ZAdd{
					ZAdd: &schema.ZAddRequest{
						Set:   []byte(`mySet`),
						Score: 0.6,
						Key:   []byte(`persistedKey`),
						AtTx:  idx.Id,
					},
				},
			},
		},
	}

	index, err := db.ExecAll(aOps)
	require.NoError(t, err)
	require.Equal(t, uint64(2), index.Id)

	list, err := db.ZScan(&schema.ZScanRequest{
		Set:     []byte(`mySet`),
		SinceTx: index.Id,
	})
	require.NoError(t, err)
	require.Equal(t, []byte(`persistedKey`), list.Entries[0].Key)
	require.Equal(t, []byte(`notPersistedKey`), list.Entries[1].Key)
}

func TestExecAllOpsEmptyList(t *testing.T) {
	db, closer := makeDb()
	defer closer()

	aOps := &schema.ExecAllRequest{
		Operations: []*schema.Op{},
	}
	_, err := db.ExecAll(aOps)
	require.Equal(t, schema.ErrEmptySet, err)
}

func TestExecAllOpsInvalidKvKey(t *testing.T) {
	db, closer := makeDb()
	defer closer()

	aOps := &schema.ExecAllRequest{
		Operations: []*schema.Op{
			{
				Operation: &schema.Op_Kv{
					Kv: &schema.KeyValue{
						Key:   []byte{},
						Value: []byte(`val`),
					},
				},
			},
		},
	}
	_, err := db.ExecAll(aOps)
	require.Equal(t, store.ErrIllegalArguments, err)
}

func TestExecAllOpsZAddKeyNotFound(t *testing.T) {
	db, closer := makeDb()
	defer closer()

	aOps := &schema.ExecAllRequest{
		Operations: []*schema.Op{
			{
				Operation: &schema.Op_ZAdd{
					ZAdd: &schema.ZAddRequest{
						Set:   []byte("set"),
						Key:   []byte(`key`),
						Score: 5.6,
						AtTx:  4,
					},
				},
			},
		},
	}
	_, err := db.ExecAll(aOps)
	require.Equal(t, store.ErrTxNotFound, err)
}

func TestExecAllOpsNilElementFound(t *testing.T) {
	db, closer := makeDb()
	defer closer()

	bOps := make([]*schema.Op, 1)
	bOps[0] = &schema.Op{
		Operation: &schema.Op_ZAdd{
			ZAdd: &schema.ZAddRequest{
				Key:   []byte(`key`),
				Score: 5.6,
				AtTx:  4,
			},
		},
	}

	_, err := db.ExecAll(&schema.ExecAllRequest{Operations: bOps})
	require.Equal(t, store.ErrIllegalArguments, err)
}

func TestSetOperationNilElementFound(t *testing.T) {
	db, closer := makeDb()
	defer closer()

	aOps := &schema.ExecAllRequest{
		Operations: []*schema.Op{
			{
				Operation: nil,
			},
		},
	}
	_, err := db.ExecAll(aOps)
	require.Error(t, err)
}

func TestExecAllOpsUnexpectedType(t *testing.T) {
	db, closer := makeDb()
	defer closer()

	aOps := &schema.ExecAllRequest{
		Operations: []*schema.Op{
			{
				Operation: &schema.Op_Unexpected{},
			},
		},
	}
	_, err := db.ExecAll(aOps)
	require.Error(t, err)
}

func TestExecAllOpsDuplicatedKey(t *testing.T) {
	db, closer := makeDb()
	defer closer()

	aOps := &schema.ExecAllRequest{
		Operations: []*schema.Op{
			{
				Operation: &schema.Op_Kv{
					Kv: &schema.KeyValue{
						Key:   []byte(`key`),
						Value: []byte(`val`),
					},
				},
			},
			{
				Operation: &schema.Op_Kv{
					Kv: &schema.KeyValue{
						Key:   []byte(`key`),
						Value: []byte(`val`),
					},
				},
			},
			{
				Operation: &schema.Op_ZAdd{
					ZAdd: &schema.ZAddRequest{
						Set:   []byte(`set`),
						Key:   []byte(`key`),
						Score: 5.6,
					},
				},
			},
		},
	}
	_, err := db.ExecAll(aOps)
	require.Equal(t, schema.ErrDuplicatedKeysNotSupported, err)
}

func TestExecAllOpsDuplicatedKeyZAdd(t *testing.T) {
	db, closer := makeDb()
	defer closer()

	aOps := &schema.ExecAllRequest{
		Operations: []*schema.Op{
			{
				Operation: &schema.Op_Kv{
					Kv: &schema.KeyValue{
						Key:   []byte(`key`),
						Value: []byte(`val`),
					},
				},
			},
			{
				Operation: &schema.Op_ZAdd{
					ZAdd: &schema.ZAddRequest{
						Key:   []byte(`key`),
						Score: 5.6,
					},
				},
			},
			{
				Operation: &schema.Op_ZAdd{
					ZAdd: &schema.ZAddRequest{
						Key:   []byte(`key`),
						Score: 5.6,
					},
				},
			},
		},
	}

	_, err := db.ExecAll(aOps)
	require.Equal(t, schema.ErrDuplicatedZAddNotSupported, err)
}

/*
func TestExecAllOpsAsynch(t *testing.T) {
	st, closer := makeStore()
	defer closer()

	aOps := &schema.Ops{
		Operations: []*schema.Op{
			{
				Operation: &schema.Op_KVs{
					KVs: &schema.KeyValue{
						Key:   []byte(`key`),
						Value: []byte(`val`),
					},
				},
			},
			{
				Operation: &schema.Op_KVs{
					KVs: &schema.KeyValue{
						Key:   []byte(`key1`),
						Value: []byte(`val1`),
					},
				},
			},
			{
				Operation: &schema.Op_ZOpts{
					ZOpts: &schema.ZAddOptions{
						Key: []byte(`key`),
						Score: &schema.Score{
							Score: 5.6,
						},
					},
				},
			},
		},
	}
	_, err := st.ExecAllOps(aOps, WithAsyncCommit(true))

	require.NoError(t, err)
}

func TestOps_ValidateErrZAddIndexMissing(t *testing.T) {
	st, closer := makeStore()
	defer closer()

	_, _ = st.Set(schema.KeyValue{
		Key:   []byte(`persistedKey`),
		Value: []byte(`persistedVal`),
	})

	// Ops payload
	aOps := &schema.Ops{
		Operations: []*schema.Op{
			{
				Operation: &schema.Op_ZOpts{
					ZOpts: &schema.ZAddOptions{
						Key: []byte(`persistedKey`),
						Score: &schema.Score{
							Score: 5.6,
						},
					},
				},
			},
		},
	}
	_, err := st.ExecAllOps(aOps)
	require.Equal(t, err, ErrZAddIndexMissing)
}

func TestStore_ExecAllOpsConcurrent(t *testing.T) {
	st, closer := makeStore()
	defer closer()

	wg := sync.WaitGroup{}
	wg.Add(10)
	for i := 1; i <= 10; i++ {
		aOps := &schema.Ops{
			Operations: []*schema.Op{},
		}
		for j := 1; j <= 10; j++ {
			key := strconv.FormatUint(uint64(j), 10)
			val := strconv.FormatUint(uint64(i), 10)
			aOp := &schema.Op{
				Operation: &schema.Op_KVs{
					KVs: &schema.KeyValue{
						Key:   []byte(key),
						Value: []byte(key),
					},
				},
			}
			aOps.Operations = append(aOps.Operations, aOp)
			float, err := strconv.ParseFloat(fmt.Sprintf("%d", j), 64)
			if err != nil {
				log.Fatal(err)
			}

			set := val
			refKey := key
			aOp = &schema.Op{
				Operation: &schema.Op_ZOpts{
					ZOpts: &schema.ZAddOptions{
						Set: []byte(set),
						Key: []byte(refKey),
						Score: &schema.Score{
							Score: float,
						},
					},
				},
			}
			aOps.Operations = append(aOps.Operations, aOp)
		}
		go func() {
			idx, err := st.ExecAllOps(aOps)
			require.NoError(t, err)
			require.NotNil(t, idx)
			wg.Done()
		}()

	}
	wg.Wait()
	for i := 1; i <= 10; i++ {
		set := strconv.FormatUint(uint64(i), 10)

		zList, err := st.ZScan(schema.ZScanOptions{
			Set: []byte(set),
		})
		require.NoError(t, err)
		require.Len(t, zList.Items, 10)
		require.Equal(t, zList.Items[i-1].Item.Value, []byte(strconv.FormatUint(uint64(i), 10)))

	}
}

func TestStore_ExecAllOpsConcurrentOnAlreadyPersistedKeys(t *testing.T) {
	dbDir := tmpDir()

	st, _ := makeStoreAt(dbDir)

	for i := 1; i <= 10; i++ {
		for j := 1; j <= 10; j++ {
			key := strconv.FormatUint(uint64(j), 10)
			_, _ = st.Set(schema.KeyValue{
				Key:   []byte(key),
				Value: []byte(key),
			})
		}
	}

	st.tree.close(true)
	st.Close()

	st, closer := makeStoreAt(dbDir)
	defer closer()

	st.tree.WaitUntil(99)

	wg := sync.WaitGroup{}
	wg.Add(10)

	gIdx := uint64(0)
	for i := 1; i <= 10; i++ {
		aOps := &schema.Ops{
			Operations: []*schema.Op{},
		}
		for j := 1; j <= 10; j++ {
			key := strconv.FormatUint(uint64(j), 10)
			val := strconv.FormatUint(uint64(i), 10)

			float, err := strconv.ParseFloat(fmt.Sprintf("%d", j), 64)
			if err != nil {
				log.Fatal(err)
			}

			set := val
			refKey := key
			aOp := &schema.Op{
				Operation: &schema.Op_ZOpts{
					ZOpts: &schema.ZAddOptions{
						Set: []byte(set),
						Key: []byte(refKey),
						Score: &schema.Score{
							Score: float,
						},
						Index: &schema.Index{Index: gIdx},
					},
				},
			}
			aOps.Operations = append(aOps.Operations, aOp)
			gIdx++
		}
		go func() {
			idx, err := st.ExecAllOps(aOps)
			require.NoError(t, err)
			require.NotNil(t, idx)
			wg.Done()
		}()
	}
	wg.Wait()

	for i := 1; i <= 10; i++ {
		set := strconv.FormatUint(uint64(i), 10)

		zList, err := st.ZScan(schema.ZScanOptions{
			Set: []byte(set),
		})
		require.NoError(t, err)
		require.Len(t, zList.Items, 10)
		require.Equal(t, zList.Items[i-1].Item.Value, []byte(strconv.FormatUint(uint64(i), 10)))
	}
}

func TestStore_ExecAllOpsConcurrentOnMixedPersistedAndNotKeys(t *testing.T) {
	// even items are stored on disk with regular sets
	// odd ones are stored inside batch operations
	// zAdd references all items

	dbDir := tmpDir()

	st, _ := makeStoreAt(dbDir)

	for i := 1; i <= 10; i++ {
		for j := 1; j <= 10; j++ {
			// even
			if j%2 == 0 {
				key := strconv.FormatUint(uint64(j), 10)
				_, _ = st.Set(schema.KeyValue{
					Key:   []byte(key),
					Value: []byte(key),
				})
			}
		}
	}

	st.tree.close(true)
	st.Close()

	st, closer := makeStoreAt(dbDir)
	defer closer()

	st.tree.WaitUntil(49)

	wg := sync.WaitGroup{}
	wg.Add(10)

	gIdx := uint64(0)
	for i := 1; i <= 10; i++ {
		aOps := &schema.Ops{
			Operations: []*schema.Op{},
		}
		for j := 1; j <= 10; j++ {
			key := strconv.FormatUint(uint64(j), 10)
			val := strconv.FormatUint(uint64(i), 10)
			var index *schema.Index

			// odd
			if j%2 != 0 {
				aOp := &schema.Op{
					Operation: &schema.Op_KVs{
						KVs: &schema.KeyValue{
							Key:   []byte(key),
							Value: []byte(key),
						},
					},
				}
				aOps.Operations = append(aOps.Operations, aOp)
				index = nil
			} else {
				index = &schema.Index{Index: gIdx}
				gIdx++
			}
			float, err := strconv.ParseFloat(fmt.Sprintf("%d", j), 64)
			if err != nil {
				log.Fatal(err)
			}

			set := val
			refKey := key
			aOp := &schema.Op{
				Operation: &schema.Op_ZOpts{
					ZOpts: &schema.ZAddOptions{
						Set: []byte(set),
						Key: []byte(refKey),
						Score: &schema.Score{
							Score: float,
						},
						Index: index,
					},
				},
			}
			aOps.Operations = append(aOps.Operations, aOp)
		}
		go func() {
			idx, err := st.ExecAllOps(aOps)
			require.NoError(t, err)
			require.NotNil(t, idx)
			wg.Done()
		}()
	}
	wg.Wait()

	for i := 1; i <= 10; i++ {
		set := strconv.FormatUint(uint64(i), 10)
		zList, err := st.ZScan(schema.ZScanOptions{
			Set: []byte(set),
		})
		require.NoError(t, err)
		require.Len(t, zList.Items, 10)
		require.Equal(t, zList.Items[i-1].Item.Value, []byte(strconv.FormatUint(uint64(i), 10)))
	}
}

func TestStore_ExecAllOpsConcurrentOnMixedPersistedAndNotOnEqualKeysAndEqualScore(t *testing.T) {
	// Inserting 100 items:
	// even items are stored on disk with regular sets
	// odd ones are stored inside batch operations
	// there are 50 batch Ops with zAdd operation for reference even items already stored
	// and in addition 50 batch Ops with 1 kv operation for odd items and zAdd operation for reference them onFly

	// items have same score. They will be returned in insertion order since key is composed by:
	// {separator}{set}{separator}{score}{key}{bit index presence flag}{index}

	dbDir := tmpDir()

	st, _ := makeStoreAt(dbDir)

	keyA := "A"

	var index *schema.Index

	for i := 1; i <= 10; i++ {
		for j := 1; j <= 10; j++ {
			// even
			if j%2 == 0 {
				val := fmt.Sprintf("%d,%d", i, j)
				index, _ = st.Set(schema.KeyValue{
					Key:   []byte(keyA),
					Value: []byte(val),
				})
				require.NotNil(t, index)
			}
		}
	}

	st.tree.close(true)
	st.Close()

	st, closer := makeStoreAt(dbDir)
	defer closer()

	st.tree.WaitUntil(49)

	wg := sync.WaitGroup{}
	wg.Add(100)

	gIdx := uint64(0)

	for i := 1; i <= 10; i++ {
		for j := 1; j <= 10; j++ {
			aOps := &schema.Ops{
				Operations: []*schema.Op{},
			}

			// odd
			if j%2 != 0 {
				val := fmt.Sprintf("%d,%d", i, j)
				aOp := &schema.Op{
					Operation: &schema.Op_KVs{
						KVs: &schema.KeyValue{
							Key:   []byte(keyA),
							Value: []byte(val),
						},
					},
				}
				aOps.Operations = append(aOps.Operations, aOp)
				index = nil
			} else {
				index = &schema.Index{Index: gIdx}
				gIdx++
			}

			float, err := strconv.ParseFloat(fmt.Sprintf("%d", j), 64)
			if err != nil {
				log.Fatal(err)
			}

			refKey := keyA
			set := strconv.FormatUint(uint64(j), 10)
			aOp := &schema.Op{
				Operation: &schema.Op_ZOpts{
					ZOpts: &schema.ZAddOptions{
						Set: []byte(set),
						Key: []byte(refKey),
						Score: &schema.Score{
							Score: float,
						},
						Index: index,
					},
				},
			}
			aOps.Operations = append(aOps.Operations, aOp)
			go func() {
				idx, err := st.ExecAllOps(aOps)
				require.NoError(t, err)
				require.NotNil(t, idx)
				wg.Done()
			}()
		}

	}
	wg.Wait()

	history, err := st.History(&schema.HistoryOptions{
		Key: []byte(keyA),
	})
	require.NoError(t, err)
	require.NotNil(t, history)
	for i := 1; i <= 10; i++ {
		set := strconv.FormatUint(uint64(i), 10)
		zList, err := st.ZScan(schema.ZScanOptions{
			Set: []byte(set),
		})
		require.NoError(t, err)
		require.NoError(t, err)
		// item are returned in insertion order since they have same score
		require.Len(t, zList.Items, 10)
	}
}

func TestExecAllOpsMonotoneTsRange(t *testing.T) {
	st, closer := makeStore()
	defer closer()

	batchSize := 100

	atomicOps := make([]*schema.Op, batchSize)

	for i := 0; i < batchSize; i++ {
		key := []byte(strconv.FormatUint(uint64(i), 10))
		value := []byte(strconv.FormatUint(uint64(batchSize+batchSize+i), 10))
		atomicOps[i] = &schema.Op{
			Operation: &schema.Op_KVs{
				KVs: &schema.KeyValue{
					Key:   key,
					Value: value,
				},
			},
		}
	}
	idx, err := st.ExecAllOps(&schema.Ops{Operations: atomicOps})
	require.NoError(t, err)
	require.Equal(t, uint64(batchSize), idx.GetIndex()+1)

	for i := 0; i < batchSize; i++ {
		item, err := st.ByIndex(schema.Index{
			Index: uint64(i),
		})
		require.NoError(t, err)
		require.Equal(t, []byte(strconv.FormatUint(uint64(batchSize+batchSize+i), 10)), item.Value)
		require.Equal(t, uint64(i), item.Index)
	}
}

func TestOps_ReferenceKeyAlreadyPersisted(t *testing.T) {
	st, closer := makeStore()
	defer closer()

	idx0, _ := st.Set(schema.KeyValue{
		Key:   []byte(`persistedKey`),
		Value: []byte(`persistedVal`),
	})

	// Ops payload
	aOps := &schema.Ops{
		Operations: []*schema.Op{
			{
				Operation: &schema.Op_ROpts{
					ROpts: &schema.ReferenceOptions{
						Reference: []byte(`myReference`),
						Key:       []byte(`persistedKey`),
						Index:     idx0,
					},
				},
			},
		},
	}
	_, err := st.ExecAllOps(aOps)
	require.NoError(t, err)

	ref, err := st.Get(schema.Key{Key: []byte(`myReference`)})

	require.NoError(t, err)
	require.NotEmptyf(t, ref, "Should not be empty")
	require.Equal(t, []byte(`persistedVal`), ref.Value, "Should have referenced item value")
	require.Equal(t, []byte(`persistedKey`), ref.Key, "Should have referenced item value")

}

func TestOps_ReferenceKeyNotYetPersisted(t *testing.T) {
	st, closer := makeStore()
	defer closer()

	// Ops payload
	aOps := &schema.Ops{
		Operations: []*schema.Op{
			{
				Operation: &schema.Op_KVs{
					KVs: &schema.KeyValue{
						Key:   []byte(`key`),
						Value: []byte(`val`),
					},
				},
			},
			{
				Operation: &schema.Op_ROpts{
					ROpts: &schema.ReferenceOptions{
						Reference: []byte(`myTag`),
						Key:       []byte(`key`),
					},
				},
			},
		},
	}
	_, err := st.ExecAllOps(aOps)
	require.NoError(t, err)

	ref, err := st.Get(schema.Key{Key: []byte(`myTag`)})

	require.NoError(t, err)
	require.NotEmptyf(t, ref, "Should not be empty")
	require.Equal(t, []byte(`val`), ref.Value, "Should have referenced item value")
	require.Equal(t, []byte(`key`), ref.Key, "Should have referenced item value")

}

func TestOps_ReferenceIndexNotExists(t *testing.T) {
	st, closer := makeStore()
	defer closer()

	// Ops payload
	aOps := &schema.Ops{
		Operations: []*schema.Op{
			{
				Operation: &schema.Op_ROpts{
					ROpts: &schema.ReferenceOptions{
						Reference: []byte(`myReference`),
						Key:       []byte(`persistedKey`),
						Index: &schema.Index{
							Index: 1234,
						},
					},
				},
			},
		},
	}
	_, err := st.ExecAllOps(aOps)
	require.Equal(t, ErrIndexNotFound, err)
}

func TestOps_ReferenceIndexMissing(t *testing.T) {
	st, closer := makeStore()
	defer closer()

	// Ops payload
	aOps := &schema.Ops{
		Operations: []*schema.Op{
			{
				Operation: &schema.Op_ROpts{
					ROpts: &schema.ReferenceOptions{
						Reference: []byte(`myReference`),
						Key:       []byte(`persistedKey`),
					},
				},
			},
		},
	}
	_, err := st.ExecAllOps(aOps)
	require.Equal(t, ErrReferenceIndexMissing, err)
}
*/
