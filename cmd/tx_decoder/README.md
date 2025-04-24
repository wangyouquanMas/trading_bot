# PumpFun AMM Transaction Decoder

This tool allows you to decode and analyze PumpFun AMM (Automated Market Maker) transactions on the Solana blockchain. You can use it to monitor specific pool addresses or decode individual transactions.

## Features

- Fetch and analyze historical transactions for a PumpFun AMM pool
- Decode individual transactions by signature
- Identify swap operations (buy/sell)
- Extract transaction details from on-chain logs
- Display summary of transaction operations
- **Automatic fallback to alternative RPC endpoints** when the primary endpoint fails
- **Retry logic** to handle temporary RPC connection issues

## Prerequisites

- Go 1.16 or higher
- Access to a Solana RPC endpoint (optional, fallbacks are provided)

## Installation

To build the transaction decoder:

```bash
cd cmd/tx_decoder
go build -o tx_decoder .
```

This will create an executable called `tx_decoder` in the current directory.

## Usage

### Basic usage

```bash
# Use the default pool address
./tx_decoder

# Analyze a specific pool
./tx_decoder decode H9d3XHfvMGfoohydEpqh4w3mopnvjCRzE9VqaiHKdqs7

# Decode a specific transaction by signature
./tx_decoder decode-tx 5SHT9PwxFE7BNmSQwU4KjAW16LQ5aEZmUvWKqSCamXKkWQBs1DcYkEv7ujWgASRUUKqYy6VsM7iTgJkgAygCVPZB
```

### Using a custom RPC endpoint

By default, the tool will try several public Solana RPC endpoints if the first one fails:

1. `https://api.mainnet-beta.solana.com`
2. `https://solana-api.projectserum.com`
3. `https://rpc.ankr.com/solana`
4. `https://solana-mainnet.g.alchemy.com/v2/demo`
5. `https://mainnet.rpcpool.com`

To use your own RPC endpoint as the primary endpoint:

```bash
# Set environment variable
export RPC_ENDPOINT="https://your-custom-endpoint.com"

# Then run the command
./tx_decoder
```

## Understanding the Output

For each transaction, the tool will:

1. Check if it involves the PumpSwap program
2. Try to determine if it's a swap operation
3. Identify buy or sell direction
4. Extract amount information if available
5. Provide a summary of the transaction

Example output:

```
Transaction 1/10: 5SHT9PwxFE7BNmSQwU4KjAW16LQ5aEZmUvWKqSCamXKkWQBs1DcYkEv7ujWgASRUUKqYy6VsM7iTgJkgAygCVPZB
Transaction signature: 5SHT9PwxFE7BNmSQwU4KjAW16LQ5aEZmUvWKqSCamXKkWQBs1DcYkEv7ujWgASRUUKqYy6VsM7iTgJkgAygCVPZB
Transaction successful
Transaction fee: 5000 lamports

Analyzing transaction logs for PumpSwap operations:
  PumpSwap program invoked
  Swap instruction detected
  Direction: BUY
  Amount: 1000000000 lamports (1 SOL)

Summary: This is a PumpSwap transaction
Operation: Swap
Direction: Buy (SOL → Token)

View full transaction details: https://solscan.io/tx/5SHT9PwxFE7BNmSQwU4KjAW16LQ5aEZmUvWKqSCamXKkWQBs1DcYkEv7ujWgASRUUKqYy6VsM7iTgJkgAygCVPZB
--------------------------------------------------
```

## Error Handling

The tool includes robust error handling features:

1. **RPC Endpoint Fallbacks**: If the primary RPC endpoint fails, the tool automatically tries several alternative public endpoints
2. **Retry Logic**: For each RPC request, the tool will retry up to 3 times with exponential backoff if a temporary error occurs
3. **Clear Error Messages**: When all attempts fail, the tool provides informative error messages to help diagnose issues

Example error output:

```
Error with primary RPC endpoint: failed to get transaction after 3 attempts: (*jsonrpc.RPCError)({ Code: 502, Message: "Bad gateway" })
Trying fallback RPC endpoints...
Trying fallback endpoint #2: https://solana-api.projectserum.com
```

## Limitations

- The tool relies on log messages to determine transaction types and directions
- Some transactions may not provide sufficient information in logs
- The tool does not decode instruction data directly, so detailed parameters may not be available
- Public RPC endpoints may have rate limits or outages

## Further Development

Potential enhancements:
- Add deeper instruction data decoding
- Support for other PumpFun operations beyond swaps
- Historical price tracking
- Export data to CSV/JSON
- WebSocket support for real-time transaction monitoring 