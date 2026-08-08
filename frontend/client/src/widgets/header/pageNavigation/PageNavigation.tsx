import styles from './PageNavigation.module.scss';
import { NavLink } from 'react-router-dom';
import { ERoutes } from '@entities/paths';
import { Button } from '@mui/material';

export function PageNavigation() {
    return (
        <nav className={styles.nav}>
            <NavLink to={ERoutes.Home}>
                <Button variant="contained" color="primary">
                    Питомец
                </Button>
            </NavLink>
            <NavLink to={ERoutes.Leaderboard}>
                <Button variant="contained" color="secondary">
                    Лидерборд
                </Button>
            </NavLink>
        </nav>
    );
}
