import type { ReactNode } from 'react';
import { CssBaseline, ThemeProvider as MuiThemeProvider, createTheme } from '@mui/material';

type AppThemeProviderProps = {
    children: ReactNode;
};

const theme = createTheme({
    cssVariables: true,
    typography: {
        fontFamily: 'Arial, sans-serif',
    },
    palette: {
        background: {
            default: '#ffffff',
        },
        text: {
            primary: '#020202',
            secondary: '#ffffff',
        },
        primary: {
            main: '#04e061',
            contrastText: '#020202',
        },
        secondary: {
            main: '#00aaff',
            contrastText: '#ffffff',
        },
    },
});

export function AppThemeProvider({ children }: AppThemeProviderProps) {
    return (
        <MuiThemeProvider theme={theme}>
            <CssBaseline />
            {children}
        </MuiThemeProvider>
    );
}
