import styles from './LeaderboardPage.module.scss';
import { Header } from '@widgets/header';

export function LeaderboardPage() {
    return (
        <>
            <Header />
            <main className={styles.LeaderboardPage}>LeaderboardPage</main>
        </>
    );
}
