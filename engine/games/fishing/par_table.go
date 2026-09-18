package fishing

type FishDef struct {
	FishID     int     `json:"fish_id"`
	Name       string  `json:"name"`
	Tier       string  `json:"tier"`
	Multiplier float64 `json:"multiplier"`
	Weight     uint32  `json:"weight"`
	TensionMs  int     `json:"tension_ms"`
}

// DefaultPARTable96 理论 RTP = 96.0000%
// 总权重: 1,000,000 | 理论总产出: 960,000
var DefaultPARTable96 = []FishDef{
	{FishID: 0, Name: "Missed", Tier: "MISS", Multiplier: 0.0, Weight: 570000, TensionMs: 300},
	{FishID: 1, Name: "Clownfish", Tier: "COMMON", Multiplier: 0.5, Weight: 160000, TensionMs: 600},
	{FishID: 2, Name: "Sardine", Tier: "COMMON", Multiplier: 1.0, Weight: 170000, TensionMs: 700},
	{FishID: 3, Name: "Flying Fish", Tier: "COMMON", Multiplier: 2.0, Weight: 50000, TensionMs: 800},
	{FishID: 4, Name: "Tuna", Tier: "RARE", Multiplier: 5.0, Weight: 30000, TensionMs: 1200},
	{FishID: 5, Name: "Manta Ray", Tier: "RARE", Multiplier: 10.0, Weight: 12000, TensionMs: 1500},
	{FishID: 6, Name: "Swordfish", Tier: "RARE", Multiplier: 20.0, Weight: 5000, TensionMs: 1800},
	{FishID: 7, Name: "Golden Turtle", Tier: "BOSS", Multiplier: 50.0, Weight: 2000, TensionMs: 2500},
	{FishID: 8, Name: "Hammerhead Shark", Tier: "BOSS", Multiplier: 100.0, Weight: 800, TensionMs: 3000},
	{FishID: 9, Name: "Giant Squid", Tier: "BOSS", Multiplier: 250.0, Weight: 160, TensionMs: 3500},
	{FishID: 10, Name: "Megalodon", Tier: "BOSS", Multiplier: 500.0, Weight: 40, TensionMs: 4500},
}
