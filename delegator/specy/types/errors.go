package types

// DONTCOVER

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// x/regulatory module sentinel errors
var (
	ErrSample = sdkerrors.Register(ModuleName, 1100, "sample error")
	//rule error define
	ErrEmptyRuleName      = sdkerrors.Register(ModuleName, 2, "rule name is empty")
	ErrEmptyRuleContent   = sdkerrors.Register(ModuleName, 3, "rule content is empty")
	ErrEmptyRuleHash      = sdkerrors.Register(ModuleName, 4, "rule hash is empty")
	ErrInvalidRuleContent = sdkerrors.Register(ModuleName, 5, "rule content is invalid")

	//binding error define
	ErrEmptyBindingName      = sdkerrors.Register(ModuleName, 6, "binding name is empty")
	ErrEmptyHash             = sdkerrors.Register(ModuleName, 7, "binding hash is empty")
	ErrEmptyBindingContent   = sdkerrors.Register(ModuleName, 8, "binding content is empty")
	ErrInvalidBindingContent = sdkerrors.Register(ModuleName, 9, "binding content is invalid")

	//regulatoryService error define
	ErrEmptyRSName        = sdkerrors.Register(ModuleName, 10, "binding name is empty")
	ErrEmptyRSEndpoint    = sdkerrors.Register(ModuleName, 11, "regulatory service endpoint is empty")
	ErrEmptyRSIpAddress   = sdkerrors.Register(ModuleName, 12, "regulatory service  ip is empty")
	ErrInvalidRSPort      = sdkerrors.Register(ModuleName, 13, "regulatory service  port is invalid")
	ErrInvalidRSIpAddress = sdkerrors.Register(ModuleName, 14, "regulatory service ip address is invalid")
	ErrDialRS             = sdkerrors.Register(ModuleName, 15, "fail dial the regulatory service")

	//registration error define
	ErrEmptyRegistrationReport = sdkerrors.Register(ModuleName, 16, "registration ias report is empty")
	ErrEmptyEnclavePK          = sdkerrors.Register(ModuleName, 17, "registration enclave pk is empty")
	//Suspicious error define
	ErrEmptySuspiciousContractAddress = sdkerrors.Register(ModuleName, 18, "Suspicious contract address is empty")
	ErrEmptySuspiciousAccount         = sdkerrors.Register(ModuleName, 19, "Suspicious account is empty")

	//compliance error define
	ErrEmptyRegistrationList = sdkerrors.Register(ModuleName, 20, "already registration is empty,dont have any compliance service")

	//operation type error define
	ErrInvalidOperationType = sdkerrors.Register(ModuleName, 21, "proposal operation type is invalid")
	ErrEntityAlreadyExists  = sdkerrors.Register(ModuleName, 22, "proposal operation entity already exists")
	ErrEntityNonExists      = sdkerrors.Register(ModuleName, 23, "proposal operation entity non-exists")

	//tx error
	ErrNotFindTxHash = sdkerrors.Register(ModuleName, 24, "not find txHash")
)
