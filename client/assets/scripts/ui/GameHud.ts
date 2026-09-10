import { _decorator, Component, Label, Button } from 'cc';
import { formatMinor, formatMultiplier } from '../util/Money';

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

    /** value: minor units (Scale 10000) */
    setBalance(value: number): void {
        if (this.balanceLabel) {
            this.balanceLabel.string = `余额: ${formatMinor(value)}`;
        }
    }

    setWin(value: number, multiplier: number): void {
        if (this.winLabel) {
            this.winLabel.string = value > 0
                ? `赢得: ${formatMinor(value)} (${formatMultiplier(multiplier)}x)`
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
