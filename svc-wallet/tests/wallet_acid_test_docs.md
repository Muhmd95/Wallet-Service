# Wallet Service ACID Test Documentation

This document describes the `wallet_acid_test.go` integration tests for the Wallet Service. These tests hit the live HTTP endpoint (`http://localhost:8000/v1`) to verify the isolated atomicity and idempotency properties of the Wallet service.

## Test Scenarios Overview

| Test Name | Concurrency | Purpose |
|-----------|-------------|---------|
| `TestSetup_CreateWallets` | 1 | Creates the base 5 wallets used throughout the test file. |
| `TestAtomicity_SingleDeposit` | 1 | Basic positive functionality check for atomic balance updates. |
| `TestAtomicity_SingleWithdrawal` | 1 | Basic negative functionality check. |
| `TestConcurrent_50ParallelDeposits` | 50 | Tests MongoDB `FindOneAndUpdate` capability to serialize 50 immediate parallel additions safely. |
| `TestConcurrent_50ParallelWithdrawals` | 50 | Tests identical operations reducing balance simultaneously. |
| `TestConcurrent_MixedDeposits...` | 70 (40+30)| Combines deposits and withdrawals into a single parallel stress test. |
| `TestIdempotency_SameKeyTwice` | 1 | Verifies that hitting the endpoint twice with the same `Idempotency-Key` does not double-credit the user. |
| `TestIdempotency_UnderConcurrency` | 20 | Verifies that if 20 duplicate requests arrive *at the exact same time*, the database enforces unique idempotency strictly. |
| `TestEdgeCase_InsufficientBalance` | 1 | Ensures balance floor guarantees block invalid debits. |
| `TestEdgeCase_DuplicateWallet...` | 1 | Ensures unique index strictly prevents multi-wallet setups per phone number. |
| `TestStress_200ConcurrentDeposits` | 200 | Maximum stress test verifying the upper limits of the database engine. |

## How the Code Works Under the Hood
The Wallet Service handles concurrency differently than the Transactions Service. Instead of `session.WithTransaction()`, it utilizes a single atomic `FindOneAndUpdate` call in MongoDB. Because it doesn't require a read-then-write loop, there are no `WriteConflict` retry issues, meaning this service can process hundreds of concurrent requests in less than a few seconds.

The query applies atomic filters (`balance >= -amount` and `processed_refs != idempotencyKey`) to strictly block double-spending or replay attacks at the database layer.

## Expected Database State
If you run `go test -v -tags=integration -count=1 ./tests/` on a **clean, empty database**, here is exactly how the `wallet_db.wallets` collection should look at the end of the suite:

### Expected Final Balances

| Phone Number | Starting Balance | Net Change Through Tests | Expected Final Balance |
|--------------|------------------|--------------------------|------------------------|
| `01511111111`| `0` | +5000 (Test2) <br> +1000 (Test7 applied once) | **6,000** |
| `01522222222`| `0` | +10000 (Setup) <br> -3000 (Test3) <br> +500 (Test8 applied once) | **7,500** |
| `01533333333`| `0` | +10000 (50 x 200 from Test4) | **10,000** |
| `01544444444`| `0` | +20000 (Setup) <br> -10000 (50 x 200 from Test5) | **10,000** |
| `01555555555`| `0` | +50000 (Setup) <br> +4000 (Test6) <br> -3000 (Test6) | **51,000** |

*Note: The remaining edge-case tests dynamically generate a random `0158XXXXXXX` phone number on every test run, so you will see a few uniquely numbered wallets in the database with a balance of `0` or `2000` (from the stress test).*
