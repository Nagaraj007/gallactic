package tests

import (
	"encoding/hex"
	"testing"

	"strings"

	"github.com/gallactic/gallactic/common/binary"
	"github.com/gallactic/gallactic/core/account"
	"github.com/gallactic/gallactic/core/account/permission"
	"github.com/gallactic/gallactic/core/evm"
	"github.com/gallactic/gallactic/core/evm/abi"
	"github.com/gallactic/gallactic/core/evm/sha3"
	"github.com/gallactic/gallactic/errors"
	"github.com/hyperledger/burrow/execution/evm/asm/bc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compiling the Permissions solidity contract at
// (generated by with 'make snatives' function) and passing to
// https://ethereum.github.io/browser-solidity (toggle details to get list)
// yields:
// Keep this updated to drive TestPermissionsContractSignatures
const compiledSigs = `
4670dc12 hasPermissions(address,uint64)
ccc2936e setPermissions(address,uint64)
30f69812 unsetPermissions(address,uint64)
`

func TestPermissionsContractSignatures(t *testing.T) {
	contract := evm.SNativeContracts()["Permissions"]

	nFuncs := len(contract.Functions())

	sigMap := idToSignatureMap()

	assert.Len(t, sigMap, nFuncs,
		"Permissions contract defines %s functions so we need %s "+
			"signatures in compiledSigs",
		nFuncs, nFuncs)

	for funcID, signature := range sigMap {
		assertFunctionIDSignature(t, contract, funcID, signature)
	}
}

func TestSNativeContractDescription_Dispatch(t *testing.T) {
	contract := evm.SNativeContracts()["Permissions"]
	caller := getAccountByName(t, "alice")
	grantee := getAccountByName(t, "bob")

	// bob has create account permission
	setPermissions(t, "bob", permission.CreateAccount)

	function, err := contract.FunctionByName("hasPermissions")
	if err != nil {
		t.Fatalf("Could not get function: %s", err)
	}
	funcID := function.ID()

	// Should fail since we have no permissions
	retValue, err := contract.Dispatch(tState, caller, bc.MustSplice(funcID[:],
		grantee.Address(), permissionsToWord256(permission.CreateAccount)), &defaultGas, tLogger)
	if !assert.Error(t, err, "Should fail due to lack of permissions") {
		return
	}
	assert.Equal(t, e.Code(err), e.ErrPermDenied)

	// Grant required permissions and dispatch should success
	caller.SetPermissions(permission.ModifyPermission)
	retValue, err = contract.Dispatch(tState, caller, bc.MustSplice(funcID[:],
		grantee.Address().Word256(), permissionsToWord256(permission.CreateAccount)), &defaultGas, tLogger)
	require.NoError(t, err)
	require.Equal(t, retValue, binary.LeftPadBytes([]byte{1}, 32))
}

func TestSNativeContractDescription_Address(t *testing.T) {
	contract := evm.NewSNativeContract("A comment",
		"CoolButVeryLongNamedContractOfDoom")
	assert.Equal(t, sha3.Sha3(([]byte)(contract.Name))[12:], contract.Address().RawBytes())
}

//
// Helpers
//
func assertFunctionIDSignature(t *testing.T, contract *evm.SNativeContractDescription,
	funcIDHex string, expectedSignature string) {
	fromHex := funcIDFromHex(t, funcIDHex)
	function, err := contract.FunctionByID(fromHex)
	assert.NoError(t, err,
		"Error retrieving SNativeFunctionDescription with ID %s", funcIDHex)
	if err == nil {
		assert.Equal(t, expectedSignature, function.Signature())
	}
}

func funcIDFromHex(t *testing.T, hexString string) abi.FunctionSelector {
	bs, err := hex.DecodeString(hexString)
	assert.NoError(t, err, "Could not decode hex string '%s'", hexString)
	if len(bs) != 4 {
		t.Fatalf("FunctionSelector must be 4 bytes but '%s' is %v bytes", hexString,
			len(bs))
	}
	return abi.FirstFourBytes(bs)
}

func permissionsToWord256(perm account.Permissions) binary.Word256 {
	return binary.Uint64ToWord256(uint64(perm))
}

// turns the solidity compiler function summary into a map to drive signature
// test
func idToSignatureMap() map[string]string {
	sigMap := make(map[string]string)
	lines := strings.Split(compiledSigs, "\n")
	for _, line := range lines {
		trimmed := strings.Trim(line, " \t")
		if trimmed != "" {
			idSig := strings.Split(trimmed, " ")
			sigMap[idSig[0]] = idSig[1]
		}
	}
	return sigMap
}
