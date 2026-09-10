import { _decorator, Component, Label, Button } from 'cc';

const { ccclass, property } = _decorator;

@ccclass('GameHud')
export class GameHud extends Component {
    @property(Label)
    balanceLabel: Label | null = null;

    @property(Label)
    winLabel: Label | null = null;

    @property(Label)
    statusLabel: Label | null = null;

    @property(Button)
    castButton: Button | null = null;

    setBalance(value: number): void {
        if (this.balanceLabel) {
            this.balanceLabel.string = `余额: ${value.toFixed(2)}`;
        }
    }

    setWin(value: number, multiplier: number): void {
        if (this.winLabel) {
            this.winLabel.string = value > 0
                ? `赢得: ${value.toFixed(2)} (${multiplier.toFixed(2)}x)`
                : '赢得: 0';
        }
    }

    setStatus(text: string): void {
        if (this.statusLabel) {
            this.statusLabel.string = text;
        }
    }

    setInteractable(enabled: boolean): void {
        if (this.castButton) {
            this.castButton.interactable = enabled;
        }
    }
}
