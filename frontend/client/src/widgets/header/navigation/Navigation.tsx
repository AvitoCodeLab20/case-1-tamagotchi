import styles from './Navigation.module.scss';
import { Link, NavLink } from 'react-router-dom';
import { Button } from '@mui/material';

export function Navigation() {
    const isRegistered = false;

    return (
        <nav className={styles.nav}>
            {!isRegistered ? (
                <>
                    <Button
                        component={Link}
                        to="/auth?mode=login"
                        sx={{
                            backgroundColor: 'var(--color-button-primary)',
                            color: 'var(--color-text-primary)',
                        }}
                    >
                        Войти
                    </Button>
                    <Button
                        sx={{
                            backgroundColor: 'var(--color-button-secondary)',
                            color: 'var(--color-text-primary)',
                        }}
                        component={Link}
                        to={'auth?mode=register'}
                    >
                        Зарегистрироваться
                    </Button>
                </>
            ) : (
                <>
                    <NavLink to="/leaderboard">
                        <Button sx={{ backgroundColor: 'var(--color-button-secondary)' }}>
                            Лидерборд
                        </Button>
                    </NavLink>
                    <NavLink to="/">
                        <Button sx={{ backgroundColor: 'var(--color-button-primary)' }}>
                            Питомец
                        </Button>
                    </NavLink>
                </>
            )}
        </nav>
    );
}
