import styles from './HomePage.module.scss';
import { Header } from '@widgets/header';

export function HomePage() {
    return (
        <>
            <Header />
            <main className={styles.homePage}>HomePage</main>
        </>
    );
}
