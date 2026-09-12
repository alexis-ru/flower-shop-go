document.getElementById('loginForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const login = document.getElementById('login').value;
    const password = document.getElementById('password').value;
    const errDiv = document.getElementById('loginError');
    errDiv.classList.add('d-none');

    try {
        const resp = await fetch('/api/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ login, password })
        });
        const data = await resp.json();
        if (resp.ok) {
            if (data.role === 'director') window.location.href = '/director.html';
            else window.location.href = '/seller.html';
        } else {
            errDiv.textContent = data.error || 'Ошибка входа';
            errDiv.classList.remove('d-none');
        }
    } catch (err) {
        errDiv.textContent = 'Сервер недоступен';
        errDiv.classList.remove('d-none');
    }
});

async function checkSession() {
    const resp = await fetch('/api/me');
    return resp.json();
}

document.getElementById('logoutBtn')?.addEventListener('click', async (e) => {
    e.preventDefault();
    await fetch('/api/logout', { method: 'POST' });
    window.location.href = '/';
});

const CITY_COORDS = { lat: 55.7558, lon: 37.6176 };

async function loadWeather() {
    const el = document.getElementById('footerWeather');
    if (!el) return;

    try {
        const url = `https://api.open-meteo.com/v1/forecast?latitude=${CITY_COORDS.lat}&longitude=${CITY_COORDS.lon}&current=temperature_2m,weathercode,wind_speed_10m&timezone=auto`;
        const resp = await fetch(url);
        if (!resp.ok) throw new Error('API недоступен');
        const data = await resp.json();

        const temp = data.current.temperature_2m;
        const code = data.current.weathercode;
        const wind = data.current.wind_speed_10m;

        // Простая логика иконок (можно расширить)
        let icon = '☀️';
        if (code >= 300 && code < 400) icon = '🌧️';
        else if (code >= 600 && code < 700) icon = '❄️';
        else if (code === 1003) icon = '🌙'; // ясно ночью

        el.innerHTML = `
            <span class="me-3">
                ${icon} ${temp}°C • Ветер ${wind} м/с
            </span>
            <span class="ms-2 text-muted">Погода: Open-Meteo</span>
        `;
    } catch (e) {
        console.error('Ошибка погоды:', e);
        el.innerHTML = '<small class="text-danger">Не удалось загрузить погоду</small>';
    }
}

// ===== Курсы валют (ЦБ РФ) =====
async function loadCurrencies() {
    const el = document.getElementById('footerCurrencies');
    if (!el) return;
    try {
        const resp = await fetch('https://www.cbr-xml-daily.ru/daily_json.js');
        const data = await resp.json();
        const usd = data.Valute.USD.Value.toFixed(2);
        const eur = data.Valute.EUR.Value.toFixed(2);
        const cny = data.Valute.CNY.Value.toFixed(2);
        el.innerHTML =
            '<span class="me-3">💵 USD: <strong>' + usd + '</strong> ₽</span>' +
            '<span class="me-3">💶 EUR: <strong>' + eur + '</strong> ₽</span>' +
            '<span>🪙 CNY: <strong>' + cny + '</strong> ₽</span>';
    } catch (e) {
        el.innerHTML = '<small>Курсы валют недоступны</small>';
    }
}

// Запуск при загрузке страницы
loadWeather();
loadCurrencies();
setInterval(loadCurrencies, 600000);
