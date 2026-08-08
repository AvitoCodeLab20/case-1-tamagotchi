import styles from './StatusBar.module.scss';
import { usePetStateSocket } from '@features/petState';
import healthIcon from '@assets/status/health.svg';
import satietyIcon from '@assets/status/satiety.svg';
import moodIcon from '@assets/status/mood.svg';
import energyIcon from '@assets/status/energy.svg';

type StatusBarProps = {
    accessToken: string | null;
};

export function StatusBar({ accessToken }: StatusBarProps) {
    const { petState, status } = usePetStateSocket(accessToken);
    const characteristics = [
        { label: 'Здоровье', value: petState?.health, icon: healthIcon },
        { label: 'Сытость', value: petState?.hunger, icon: satietyIcon },
        { label: 'Настроение', value: petState?.happiness, icon: moodIcon },
        { label: 'Энергия', value: petState?.energy, icon: energyIcon },
    ];

    return (
        <div className={styles.statusBar} aria-label="Характеристики питомца" data-status={status}>
            {characteristics.map(({ label, value, icon }) => (
                <div className={styles.characteristic} key={label} title={label}>
                    <img className={styles.icon} src={icon} alt="" aria-hidden="true" />
                    <span className={styles.value}>{value ?? '—'}</span>
                </div>
            ))}
        </div>
    );
}
