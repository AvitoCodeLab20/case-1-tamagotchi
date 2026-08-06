import styles from './AuthPage.module.scss';
import { useSearchParams } from 'react-router-dom';
import { Button } from '@mui/material';
import { AuthForm } from '@features/authForm';

type AuthMode = 'login' | 'register';

export function AuthPage() {
    const [searchParams, setSearchParams] = useSearchParams();
    const mode: AuthMode = searchParams.get('mode') === 'register' ? 'register' : 'login';
    const isRegisterMode = mode === 'register';

    const toggleMode = () => {
        setSearchParams({ mode: isRegisterMode ? 'login' : 'register' });
    };

    return (
        <main className={styles.authPage}>
            <Button className={styles.modeButton} variant="outlined" onClick={toggleMode}>
                {isRegisterMode ? 'К входу' : 'К регистрации'}
            </Button>

            <AuthForm isRegisterMode={isRegisterMode} />
        </main>
    );
}
