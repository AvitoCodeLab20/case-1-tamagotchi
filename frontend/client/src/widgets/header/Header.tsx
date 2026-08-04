import styles from './Header.module.scss';
import { Navigation } from '@widgets/header/navigation';
export function Header() {
    return (
        <>
            <header className={styles.header}>
                <Navigation />
            </header>
        </>
    );
}
