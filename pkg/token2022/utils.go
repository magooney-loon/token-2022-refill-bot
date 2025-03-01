package token2022

import (
	"encoding/binary"
	"fmt"

	"github.com/gagliardetto/solana-go"
)

const (
	// Extension type identifiers
	ExtensionTransferFee           = 1
	ExtensionInterestBearing       = 2
	ExtensionPermanentDelegate     = 3
	ExtensionConfidentialTransfers = 4
	ExtensionDefaultAccountState   = 5
	ExtensionMemoTransfer          = 6
	ExtensionNonTransferable       = 7
	ExtensionInterestBearingConfig = 8

	// Base account data size
	MintAccountSize = 82
)

type authorities struct {
	FreezeAuthority string
	MintAuthority   string
}

// parseTransferFee parses the transfer fee extension data
func (c *Client) parseTransferFee(data []byte) (*TransferFee, error) {
	if len(data) < MintAccountSize+16 {
		return nil, fmt.Errorf("data too short for transfer fee")
	}

	// Skip base mint data
	data = data[MintAccountSize:]

	// Check extension type
	if binary.LittleEndian.Uint16(data) != ExtensionTransferFee {
		return nil, fmt.Errorf("not a transfer fee extension")
	}

	// Parse transfer fee data
	basisPoints := binary.LittleEndian.Uint16(data[2:])
	maximumFee := binary.LittleEndian.Uint64(data[4:])
	collector := solana.PublicKeyFromBytes(data[12:44])

	return &TransferFee{
		BasisPoints:     basisPoints,
		MaximumFee:      maximumFee,
		CollectorWallet: collector,
	}, nil
}

// parseInterestRate parses the interest rate extension data
func (c *Client) parseInterestRate(data []byte) (*InterestRate, error) {
	if len(data) < MintAccountSize+24 {
		return nil, fmt.Errorf("data too short for interest rate")
	}

	// Skip base mint data
	data = data[MintAccountSize:]

	// Check extension type
	if binary.LittleEndian.Uint16(data) != ExtensionInterestBearing {
		return nil, fmt.Errorf("not an interest bearing extension")
	}

	// Parse interest rate data
	currentRate := binary.LittleEndian.Uint16(data[2:])
	apy := float64(currentRate) / 100.0 // Convert basis points to percentage
	lastUpdateSlot := binary.LittleEndian.Uint64(data[16:])

	return &InterestRate{
		CurrentRate:    currentRate,
		APY:            apy,
		LastUpdateSlot: lastUpdateSlot,
	}, nil
}

// parsePermanentDelegate parses the permanent delegate extension data
func (c *Client) parsePermanentDelegate(data []byte) (string, error) {
	if len(data) < MintAccountSize+34 {
		return "", fmt.Errorf("data too short for permanent delegate")
	}

	// Skip base mint data
	data = data[MintAccountSize:]

	// Check extension type
	if binary.LittleEndian.Uint16(data) != ExtensionPermanentDelegate {
		return "", fmt.Errorf("not a permanent delegate extension")
	}

	// Parse delegate pubkey
	delegate := solana.PublicKeyFromBytes(data[2:34])
	return delegate.String(), nil
}

// parseAuthorities parses the mint and freeze authorities
func (c *Client) parseAuthorities(data []byte) (*authorities, error) {
	if len(data) < MintAccountSize {
		return nil, fmt.Errorf("data too short for authorities")
	}

	// Authorities are in the base mint data
	// Mint authority is at offset 0
	// Freeze authority is at offset 36
	mintAuth := solana.PublicKeyFromBytes(data[0:32])
	freezeAuth := solana.PublicKeyFromBytes(data[36:68])

	return &authorities{
		MintAuthority:   mintAuth.String(),
		FreezeAuthority: freezeAuth.String(),
	}, nil
}
