(async () => {
    const me = await checkSession();
    if (!me.authenticated || me.role !== 'director') {
        window.location.href = '/';
        return;
    }
    document.getElementById('userName').textContent = me.name;
    loadUsers();
    loadDirectorFlowers();
})();

document.querySelectorAll('#dirTabs .nav-link').forEach(btn => {
    btn.addEventListener('click', () => {
        document.querySelectorAll('#dirTabs .nav-link').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        document.getElementById('tab-sellers').classList.add('d-none');
        document.getElementById('tab-flowers').classList.add('d-none');
        document.getElementById('tab-' + btn.dataset.tab).classList.remove('d-none');
    });
});

async function loadUsers() {
    const resp = await fetch('/api/users');
    const users = await resp.json();
    const tbody = document.getElementById('usersTable');
    tbody.innerHTML = '';
    users.forEach(u => {
        let badge = '<span class="badge badge-active">Работает</span>';
        if (u.status === 'blocked') badge = '<span class="badge badge-blocked">Заблокирован</span>';
        if (u.status === 'fired')   badge = '<span class="badge badge-fired">Уволен</span>';

        let actions = '';
        if (u.role === 'seller') {
            actions = `
                <button class="btn btn-warning btn-sm me-1" onclick="userAction(${u.id},'block')">Блок</button>
                <button class="btn btn-info btn-sm me-1" onclick="userAction(${u.id},'unblock')">Разблок</button>
                <button class="btn btn-secondary btn-sm me-1" onclick="userAction(${u.id},'fire')">Уволить</button>
                <button class="btn btn-danger btn-sm" onclick="userAction(${u.id},'delete')">Удалить</button>`;
        } else {
            actions = '<span class="text-muted">Директор</span>';
        }

        tbody.innerHTML += `<tr>
            <td>${u.id}</td>
            <td>${esc(u.full_name)}</td>
            <td>${esc(u.login)}</td>
            <td>${u.role === 'director' ? 'Директор' : 'Продавец'}</td>
            <td>${badge}</td>
            <td>${fmtDate(u.registration_date)}</td>
            <td>${fmtDate(u.block_date)}</td>
            <td>${fmtDate(u.dismissal_date)}</td>
            <td>${actions}</td>
        </tr>`;
    });
}

window.userAction = async (id, action) => {
    if (action === 'delete' && !confirm('Удалить продавца безвозвратно?')) return;
    await fetch(`/api/users/${id}/${action}`, { method: 'PUT' });
    loadUsers();
};

document.getElementById('addUserBtn').addEventListener('click', async () => {
    const body = {
        full_name: document.getElementById('newFullName').value,
        login:     document.getElementById('newLogin').value,
        password:  document.getElementById('newPassword').value
    };
    const errDiv = document.getElementById('addUserError');
    errDiv.classList.add('d-none');
    const resp = await fetch('/api/users/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
    });
    if (resp.ok) {
        bootstrap.Modal.getInstance(document.getElementById('addUserModal')).hide();
        document.getElementById('newFullName').value = '';
        document.getElementById('newLogin').value = '';
        document.getElementById('newPassword').value = '';
        loadUsers();
    } else {
        const d = await resp.json();
        errDiv.textContent = d.error || 'Ошибка';
        errDiv.classList.remove('d-none');
    }
});

async function loadDirectorFlowers() {
    const resp = await fetch('/api/flowers');
    const flowers = await resp.json();
    const tbody = document.getElementById('dirFlowersTable');
    tbody.innerHTML = '';
    flowers.forEach(f => {
        tbody.innerHTML += `<tr>
            <td>${f.id}</td>
            <td>${esc(f.seller_name)}</td>
            <td>${esc(f.name)}</td>
            <td>${f.quantity}</td>
            <td>${f.arrival_date || '—'}</td>
            <td>${f.sale_date || '—'}</td>
        </tr>`;
    });
}

function esc(s) { return (s || '').replace(/</g, '&lt;'); }
function fmtDate(d) { return d ? new Date(d).toLocaleDateString('ru-RU') : '—'; }
