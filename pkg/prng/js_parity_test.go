package prng

// js_parity_test.go —— 客户端验算脚本与服务端开奖口径的一致性校验
//
// 为什么必须有这个测试：provably-fair 的价值在于「客户端能独立算出服务端的
// 派彩」。服务端口径是 pkg/prng（roll→PAR 表赔付），客户端镜像在
// web/shared/prng-money.js。两处一旦漂移，玩家验算会发现金额对不上，
// 而这种漂移**编译期完全看不出来**——只能真跑一遍逐局比对。
//
// 放在 pkg/prng 而不是 pkg/par 的原因：这里要调用结算路径本身（Spin），
// 而 pkg/par 是被 pkg/prng 依赖的叶子包，反向依赖会成环。
//
// 依赖 node；没有 node 时自动跳过，因此不影响常规 `go test ./...`。
//
//	go test ./pkg/prng/ -run TestJSMirror -v

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"fastgame/pkg/money"
	"fastgame/pkg/par"
)

// parityDriver 是交给 node 执行的比对驱动（见 testdata/parity_driver.mjs）。
// 放在单独文件里而不是 Go 原始字符串里：JS 的反引号会终止 Go 的反引号字符串。
//
//go:embed testdata/parity_driver.mjs
var parityDriver string

const (
	parityServerSeed = "parity-server-seed"
	parityClientSeed = "parity-client-seed"
)

type jsParityCase struct {
	Nonce           string `json:"nonce"`
	Roll            uint64 `json:"roll"`
	MultiplierMinor int64  `json:"multiplierMinor"`
	WinMinor        int64  `json:"winMinor"`
	BetMinor        int64  `json:"betMinor"`
}

func TestJSMirrorMatchesServerOutcome(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("跳过：未安装 node，无法执行客户端验算脚本比对")
	}

	modulePath, err := filepath.Abs(filepath.Join("..", "..", "web", "shared", "prng-money.js"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(modulePath); err != nil {
		t.Skipf("跳过：找不到客户端验算脚本 %s", modulePath)
	}

	// 服务端真实结算路径产出期望值，再用客户端脚本复现同一批局
	const rounds = 500
	bet := money.FromMajor(10)
	engine := NewEngine("default")
	defer engine.Release()

	cases := make([]jsParityCase, 0, rounds)
	seenTiers := map[string]int{}
	for i := 0; i < rounds; i++ {
		nonce := fmt.Sprintf("parity-%d", i)
		out, proof, err := engine.Spin(parityServerSeed, parityClientSeed, nonce, bet)
		if err != nil {
			t.Fatal(err)
		}
		seenTiers[out.FishTier]++
		cases = append(cases, jsParityCase{
			Nonce:           nonce,
			Roll:            proof.Roll,
			MultiplierMinor: int64(out.Multiplier),
			WinMinor:        out.WinAmount.Minor(),
			BetMinor:        bet.Minor(),
		})
	}
	// 样本必须覆盖到非空杆档，否则这个比对可能"因为全是 0"而假通过
	if seenTiers[par.TierMiss] == rounds {
		t.Fatal("比对样本全是空杆，无法证明赔付档位一致")
	}
	t.Logf("样本档位分布: %v", seenTiers)

	tmp := t.TempDir()
	casesFile := filepath.Join(tmp, "cases.json")
	driverFile := filepath.Join(tmp, "driver.mjs")

	data, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(casesFile, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(driverFile, []byte(parityDriver), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(node, driverFile, modulePath, casesFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("客户端验算脚本比对失败：%v\n%s", err, out)
	}
	t.Logf("%s", out)
}
