import styles from './Header.module.scss';
import { LoginNavigation } from './loginNavigation';
import { PageNavigation } from './pageNavigation';
import { useSessionStore } from '@entities/session';
import { StatusBar } from './statusBar';
import { Stack } from '@mui/material';
export function Header() {
    const accessToken = useSessionStore((state) => state.accessToken);
    const isAuthenticated = useSessionStore((state) => state.status === 'authenticated');

    return (
        <>
            <header className={styles.header}>
                <Stack direction="row" spacing="5%">
                    <PageNavigation />
                    {isAuthenticated && <StatusBar accessToken={accessToken} />}
                </Stack>
                <LoginNavigation />
            </header>
        </>
    );
}
