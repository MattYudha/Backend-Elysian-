package config

// BlockchainConfig holds blockchain connection settings
type BlockchainConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	RPCURL          string `mapstructure:"rpc_url"`
	ContractAddr    string `mapstructure:"contract_addr"`
	PrivateKey      string `mapstructure:"private_key"`
	Network         string `mapstructure:"network"`
	NFTContractAddr string `mapstructure:"nft_contract_addr"`
	PinataAPIKey    string `mapstructure:"pinata_api_key"`
	PinataSecretKey string `mapstructure:"pinata_secret_key"`
}
