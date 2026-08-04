import styles from './Navigation.module.scss';
import { Link, NavLink } from 'react-router-dom';
import { Button } from '@mui/material';

export function Navigation() {
    const isRegistered = true;

    return (
        <nav className={styles.nav}>
            {!isRegistered ? (
                <>
                    <Button
                        component={Link}
                        to="/auth?mode=login"
                        variant="contained"
                        color="primary"
                    >
                        Войти
                    </Button>
                    <Button
                        variant="contained"
                        color="secondary"
                        component={Link}
                        to={'auth?mode=register'}
                    >
                        Зарегистрироваться
                    </Button>
                </>
            ) : (
                <>
                    <NavLink to="/leaderboard">
                        <Button variant="contained" color="primary">
                            Лидерборд
                        </Button>
                    </NavLink>
                    <NavLink to="/">
                        <Button variant="contained" color="secondary">
                            Питомец
                        </Button>
                    </NavLink>
                </>
            )}
        </nav>
    );
}
