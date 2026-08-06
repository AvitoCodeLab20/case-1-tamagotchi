import styles from './Navigation.module.scss';
import { Link, NavLink } from 'react-router-dom';
import { ERoutes } from '@entities/paths';
import { Button } from '@mui/material';

export function Navigation() {
    const isRegistered = true;

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
                    <NavLink to={ERoutes.Leaderboard}>
                        <Button variant="contained" color="primary">
                            Лидерборд
                        </Button>
                    </NavLink>
                    <NavLink to={ERoutes.Home}>
                        <Button variant="contained" color="secondary">
                            Питомец
                        </Button>
                    </NavLink>
                </>
            )}
        </nav>
    );
}
