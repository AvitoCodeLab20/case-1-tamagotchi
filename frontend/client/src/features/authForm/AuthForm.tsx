import { Button, Paper, TextField, Typography } from '@mui/material';
import styles from '@features/authForm/AuthForm.module.scss';
import type { SyntheticEvent } from 'react';

type AuthFormProps = {
    isRegisterMode: boolean;
};

export function AuthForm(props: AuthFormProps) {
    const { isRegisterMode } = props;

    const handleSubmit = function (event: SyntheticEvent<HTMLFormElement>) {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        console.log(data);
    };

    return (
        <Paper className={styles.authForm} component="form" elevation={3} onSubmit={handleSubmit}>
            <Typography component="h1" variant="h4" align="center">
                {isRegisterMode ? 'Регистрация' : 'Вход'}
            </Typography>

            <TextField
                label="Email"
                name="email"
                type="email"
                autoComplete="email"
                required
                fullWidth
                autoFocus
            />
            <TextField
                label="Пароль"
                name="password"
                type="password"
                autoComplete={isRegisterMode ? 'new-password' : 'current-password'}
                required
                fullWidth
            />
            {isRegisterMode && (
                <TextField
                    label="Повторите пароль"
                    name="confirmPassword"
                    type="password"
                    autoComplete="new-password"
                    required
                    fullWidth
                />
            )}

            <Button type="submit" variant="contained" size="large" fullWidth>
                {isRegisterMode ? 'Зарегистрироваться' : 'Войти'}
            </Button>
        </Paper>
    );
}
