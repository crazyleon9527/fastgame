import { _decorator, Component, Label } from 'cc';
import { BigWinSocket, BigWinPayload } from '../network/BigWinSocket';

const { ccclass, property } = _decorator;

@ccclass('BigWinMarquee')
export class BigWinMarquee extends Component {
    @property(Label)
    marqueeLabel: Label | null = null;

    private socket = new BigWinSocket();

    onLoad(): void {
        this.socket.connect((payload: BigWinPayload) => this.show(payload));
    }

    onDestroy(): void {
        this.socket.disconnect();
    }

    private show(payload: BigWinPayload): void {
        if (!this.marqueeLabel) {
            return;
        }
        this.marqueeLabel.string =
            `🎉 玩家 ${payload.userId} 获得 ${payload.winAmount.toFixed(2)} (${payload.multiplier.toFixed(0)}x)!`;
    }
}
