import styles from './LoginNavigation.module.scss';
import { Link } from 'react-router-dom';
import { ERoutes } from '@entities/paths';
import { Button } from '@mui/material';
import { logout, useSessionStore } from '@entities/session';

export function LoginNavigation() {
    const isRegistered = useSessionStore((state) => state.status !== 'authenticated');
    console.log(isRegistered);
    return (
        <nav className={styles.nav}>
            {!isRegistered ? (
                <>
                    <Button
                        component={Link}
                        to={`${ERoutes.Auth}?mode=login`}
                        variant="contained"
                        color="primary"
                    >
                        Войти
                    </Button>
                    <Button
                        variant="contained"
                        color="secondary"
                        component={Link}
                        to={`${ERoutes.Auth}?mode=register`}
                    >
                        Зарегистрироваться
                    </Button>
                </>
            ) : (
                <>
                    <Button variant="contained" onClick={() => void logout()}>
                        Выйти
                    </Button>
                </>
            )}
        </nav>
    );
}
