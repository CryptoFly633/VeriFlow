package types

// Governance module event types
const (
	//contract

	//register
	EventTypeRegisterProposal         = "register_proposal"
	AttributeKeyRegulatoryServiceName = "regulatory_service_name"

	//rule
	AttributeKeyRuleName    = "rule_name"
	AttributeKeyRuleContent = "rule_content"
	AttributeKeyRuleHash    = "rule_hash"
	EventTypeRuleProposal   = "rule_proposal"

	//binding
	AttributeKeyBindingName           = "binding_name"
	AttributeKeyBindingContent        = "binding_content"
	AttributeKeyBindingHash           = "binding_hash"
	AttributeKeyBindingRuleFilesNames = "binding_rule_files_names"
	EventTypeBindingProposal          = "binding_proposal"

	//relation
	AttributeKeyContractAddress = "contract_address"
	EventTypeRelationProposal   = "relation_proposal"

	//Suspicious
	EventTypeSuspiciousUpdate      = "suspicious_update"
	AttributeKeySuspiciousAccounts = "suspicious_accounts"

	//operation Type attribute key
	AttributeKeyOperationType = "operation_type"
)
