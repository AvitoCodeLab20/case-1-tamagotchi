import styles from './StatusBar.module.scss';
import satietyIcon from '@assets/status/satiety.svg';
import moodIcon from '@assets/status/mood.svg';
import energyIcon from '@assets/status/energy.svg';

const characteristics = [
    { value: 78, icon: satietyIcon },
    { value: 92, icon: moodIcon },
    { value: 65, icon: energyIcon },
];

export function StatusBar() {
    return (
        <div className={styles.statusBar} aria-label="Характеристики питомца">
            {characteristics.map(({ value, icon }) => (
                <div className={styles.characteristic} key={icon}>
                    <img className={styles.icon} src={icon} alt="" aria-hidden="true" />
                    <span className={styles.value}>{value}</span>
                </div>
            ))}
        </div>
    );
}
