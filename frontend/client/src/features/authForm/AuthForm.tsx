import { Alert, Button, Paper, TextField, Typography } from '@mui/material';
import styles from '@features/authForm/AuthForm.module.scss';
import type { SyntheticEvent } from 'react';
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { signIn, signUp } from '@entities/session';
import { ERoutes } from '@entities/paths';

type AuthFormProps = {
    isRegisterMode: boolean;
};

export function AuthForm(props: AuthFormProps) {
    const { isRegisterMode } = props;
    const navigate = useNavigate();
    const [error, setError] = useState<string | null>(null);
    const [isSubmitting, setIsSubmitting] = useState(false);

    const handleSubmit = async function (event: SyntheticEvent<HTMLFormElement>) {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        const email = data.get('email');
        const password = data.get('password');
        const confirmPassword = data.get('confirmPassword');

        if (typeof email !== 'string' || typeof password !== 'string') {
            return;
        }

        if (isRegisterMode && password !== confirmPassword) {
            setError('Пароли не совпадают.');
            return;
        }

        setError(null);
        setIsSubmitting(true);

        try {
            if (isRegisterMode) {
                await signUp({ email, password });
            } else {
                await signIn({ email, password });
            }
            navigate(ERoutes.Home, { replace: true });
        } catch {
            setError('Не удалось выполнить вход. Проверьте данные и попробуйте ещё раз.');
        } finally {
            setIsSubmitting(false);
        }
    };

    return (
        <Paper className={styles.authForm} component="form" elevation={3} onSubmit={handleSubmit}>
            <Typography component="h1" variant="h4" align="center">
                {isRegisterMode ? 'Регистрация' : 'Вход'}
            </Typography>

            {error && <Alert severity="error">{error}</Alert>}

            <TextField
                label="Email"
                name="email"
                type="email"
                autoComplete="email"
                required
                fullWidth
                autoFocus
                disabled={isSubmitting}
            />
            <TextField
                label="Пароль"
                name="password"
                type="password"
                autoComplete={isRegisterMode ? 'new-password' : 'current-password'}
                required
                fullWidth
                disabled={isSubmitting}
            />
            {isRegisterMode && (
                <TextField
                    label="Повторите пароль"
                    name="confirmPassword"
                    type="password"
                    autoComplete="new-password"
                    required
                    fullWidth
                    disabled={isSubmitting}
                />
            )}

            <Button
                type="submit"
                variant="contained"
                size="large"
                fullWidth
                disabled={isSubmitting}
            >
                {isRegisterMode ? 'Зарегистрироваться' : 'Войти'}
            </Button>
        </Paper>
    );
}
