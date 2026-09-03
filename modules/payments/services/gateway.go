package services

// PaymentGateway interface - implement this for each payment provider
type PaymentGateway interface {
	// GetName returns the gateway identifier (e.g., "bitgo", "tron", "coinbase")
	GetName() string

	// GetSupportedCoins returns list of supported coins
	GetSupportedCoins() []string

	// CreateAddress creates a new deposit address for a user
	CreateAddress(userID, coin, label string) (address string, err error)

	// GetTransaction fetches transaction details
	GetTransaction(txid string) (*GatewayTransaction, error)

	// IsConfigured returns true if gateway is properly configured
	IsConfigured() bool
}

// GatewayTransaction - normalized transaction from any gateway
type GatewayTransaction struct {
	TxID          string
	Confirmations int
	Outputs       []GatewayOutput
	FromExternal  bool // true if deposit from outside
}

type GatewayOutput struct {
	Address string
	Amount  float64 // in coin units (BTC, USDT, etc.)
	IsOurs  bool    // belongs to our wallet
}

// GatewayRegistry holds all available payment gateways
type GatewayRegistry struct {
	gateways map[string]PaymentGateway
}

func NewGatewayRegistry() *GatewayRegistry {
	return &GatewayRegistry{
		gateways: make(map[string]PaymentGateway),
	}
}

func (r *GatewayRegistry) Register(gateway PaymentGateway) {
	r.gateways[gateway.GetName()] = gateway
}

func (r *GatewayRegistry) Get(name string) PaymentGateway {
	return r.gateways[name]
}

func (r *GatewayRegistry) GetForCoin(coin string) PaymentGateway {
	for _, gw := range r.gateways {
		for _, c := range gw.GetSupportedCoins() {
			if c == coin {
				return gw
			}
		}
	}
	return nil
}

func (r *GatewayRegistry) ListAvailable() []PaymentGateway {
	var available []PaymentGateway
	for _, gw := range r.gateways {
		if gw.IsConfigured() {
			available = append(available, gw)
		}
	}
	return available
}

func (r *GatewayRegistry) ListAllCoins() []string {
	coinMap := make(map[string]bool)
	for _, gw := range r.gateways {
		if gw.IsConfigured() {
			for _, c := range gw.GetSupportedCoins() {
				coinMap[c] = true
			}
		}
	}
	var coins []string
	for c := range coinMap {
		coins = append(coins, c)
	}
	return coins
}
