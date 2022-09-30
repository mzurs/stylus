package arbtest

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/offchainlabs/nitro/arbnode"
	"github.com/offchainlabs/nitro/util/testhelpers"
)

type callTxArgs struct {
	From       *common.Address `json:"from"`
	To         *common.Address `json:"to"`
	Gas        *hexutil.Uint64 `json:"gas"`
	GasPrice   *hexutil.Big    `json:"gasPrice"`
	Value      *hexutil.Big    `json:"value"`
	Data       *hexutil.Bytes  `json:"data"`
	Aggregator *common.Address `json:"aggregator"`
}
type traceAction struct {
	CallType string          `json:"callType,omitempty"`
	From     common.Address  `json:"from"`
	Gas      hexutil.Uint64  `json:"gas"`
	Input    *hexutil.Bytes  `json:"input,omitempty"`
	Init     hexutil.Bytes   `json:"init,omitempty"`
	To       *common.Address `json:"to,omitempty"`
	Value    *hexutil.Big    `json:"value"`
}

type traceCallResult struct {
	Address *common.Address `json:"address,omitempty"`
	Code    *hexutil.Bytes  `json:"code,omitempty"`
	GasUsed hexutil.Uint64  `json:"gasUsed"`
	Output  *hexutil.Bytes  `json:"output,omitempty"`
}

type traceFrame struct {
	Action              traceAction      `json:"action"`
	BlockHash           *hexutil.Bytes   `json:"blockHash,omitempty"`
	BlockNumber         *uint64          `json:"blockNumber,omitempty"`
	Result              *traceCallResult `json:"result,omitempty"`
	Error               *string          `json:"error,omitempty"`
	Subtraces           int              `json:"subtraces"`
	TraceAddress        []int            `json:"traceAddress"`
	TransactionHash     *hexutil.Bytes   `json:"transactionHash,omitempty"`
	TransactionPosition *uint64          `json:"transactionPosition,omitempty"`
	Type                string           `json:"type"`
}

type traceResult struct {
	Output             hexutil.Bytes     `json:"output"`
	StateDiff          *int              `json:"stateDiff"`
	Trace              []traceFrame      `json:"trace"`
	VmTrace            *int              `json:"vmTrace"`
	DestroyedContracts *[]common.Address `json:"destroyedContracts"`
}

type callTraceRequest struct {
	callArgs   callTxArgs
	traceTypes []string
}

func (at *callTraceRequest) UnmarshalJSON(b []byte) error {
	fields := []interface{}{&at.callArgs, &at.traceTypes}
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	if len(fields) != 2 {
		return errors.New("expected two arguments per call")
	}
	return nil
}

func (at *callTraceRequest) MarshalJSON() ([]byte, error) {
	fields := []interface{}{&at.callArgs, &at.traceTypes}
	data, err := json.Marshal(&fields)
	return data, err
}

type filterRequest struct {
	FromBlock   *rpc.BlockNumberOrHash `json:"fromBlock"`
	ToBlock     *rpc.BlockNumberOrHash `json:"toBlock"`
	FromAddress *[]common.Address      `json:"fromAddress"`
	ToAddress   *[]common.Address      `json:"toAddress"`
	After       *uint64                `json:"after"`
	Count       *uint64                `json:"count"`
}

type ArbTraceAPIStub struct {
	t *testing.T
}

func (s *ArbTraceAPIStub) Call(ctx context.Context, callArgs callTxArgs, traceTypes []string, blockNum rpc.BlockNumberOrHash) (*traceResult, error) {
	return &traceResult{}, nil
}

func (s *ArbTraceAPIStub) CallMany(ctx context.Context, calls []*callTraceRequest, blockNum rpc.BlockNumberOrHash) ([]*traceResult, error) {
	return []*traceResult{&traceResult{}}, nil
}

func (s *ArbTraceAPIStub) ReplayBlockTransactions(ctx context.Context, blockNum rpc.BlockNumberOrHash, traceTypes []string) ([]*traceResult, error) {
	return []*traceResult{&traceResult{}}, nil
}

func (s *ArbTraceAPIStub) ReplayTransaction(ctx context.Context, txHash hexutil.Bytes, traceTypes []string) (*traceResult, error) {
	return &traceResult{}, nil
}

func (s *ArbTraceAPIStub) Transaction(ctx context.Context, txHash hexutil.Bytes) ([]traceFrame, error) {
	return []traceFrame{traceFrame{}}, nil
}

func (s *ArbTraceAPIStub) Get(ctx context.Context, txHash hexutil.Bytes, path []hexutil.Uint64) (*traceFrame, error) {
	return &traceFrame{}, nil
}

func (a *ArbTraceAPIStub) Block(ctx context.Context, blockNum rpc.BlockNumberOrHash) ([]traceFrame, error) {
	return []traceFrame{traceFrame{}}, nil
}

func (a *ArbTraceAPIStub) Filter(ctx context.Context, filter *filterRequest) ([]traceFrame, error) {
	return []traceFrame{traceFrame{}}, nil
}

func TestArbTraceForwarding(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ipcPath := filepath.Join(t.TempDir(), "redirect.ipc")
	var apis []rpc.API
	apis = append(apis, rpc.API{
		Namespace: "arbtrace",
		Version:   "1.0",
		Service:   &ArbTraceAPIStub{t: t},
		Public:    false,
	})
	listener, srv, err := rpc.StartIPCEndpoint(ipcPath, apis)
	testhelpers.RequireImpl(t, err)
	defer srv.Stop()
	defer listener.Close()

	nodeConfig := arbnode.ConfigDefaultL1Test()
	nodeConfig.RPC.ClassicRedirect = ipcPath
	nodeConfig.RPC.ClassicRedirectTimeout = time.Second
	_, _, _, l2stack, _, _, _, l1stack := createTestNodeOnL1WithConfigImpl(t, ctx, true, nodeConfig, nil, nil)
	defer requireClose(t, l1stack)
	defer requireClose(t, l2stack)

	l2rpc, _ := l2stack.Attach()
	txArgs := callTxArgs{}
	traceTypes := []string{"trace"}
	blockNum := rpc.BlockNumberOrHash{}
	traceRequests := make([]*callTraceRequest, 1)
	traceRequests[0] = &callTraceRequest{callArgs: callTxArgs{}, traceTypes: traceTypes}
	txHash := hexutil.Bytes{}
	path := []hexutil.Uint64{}
	filter := filterRequest{}
	var result traceResult
	err = l2rpc.CallContext(ctx, &result, "arbtrace_call", txArgs, traceTypes, blockNum)
	testhelpers.RequireImpl(t, err)
	var results []*traceResult
	err = l2rpc.CallContext(ctx, &results, "arbtrace_callMany", traceRequests, blockNum)
	testhelpers.RequireImpl(t, err)
	err = l2rpc.CallContext(ctx, &results, "arbtrace_replayBlockTransactions", blockNum, traceTypes)
	testhelpers.RequireImpl(t, err)
	err = l2rpc.CallContext(ctx, &result, "arbtrace_replayTransaction", txHash, traceTypes)
	testhelpers.RequireImpl(t, err)
	var frames []traceFrame
	err = l2rpc.CallContext(ctx, &frames, "arbtrace_transaction", txHash)
	testhelpers.RequireImpl(t, err)
	var frame traceFrame
	err = l2rpc.CallContext(ctx, &frame, "arbtrace_get", txHash, path)
	testhelpers.RequireImpl(t, err)
	err = l2rpc.CallContext(ctx, &frames, "arbtrace_block", blockNum)
	testhelpers.RequireImpl(t, err)
	err = l2rpc.CallContext(ctx, &frames, "arbtrace_filter", filter)
	testhelpers.RequireImpl(t, err)
}
