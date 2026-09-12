(async () => {
    const me = await checkSession();
    if (!me.authenticated || me.role !== 'seller') {
        window.location.href = '/';
        return;
    }
    document.getElementById('userName').textContent = me.name;
    loadSellerFlowers();
})();

let editingFlowerId = null;

async function loadSellerFlowers() {
    const resp = await fetch('/api/flowers');
    const flowers = await resp.json();
    const tbody = document.getElementById('sellerFlowersTable');
    tbody.innerHTML = '';
    flowers.forEach(f => {
        tbody.innerHTML += `<tr>
            <td>${f.id}</td>
            <td>${esc(f.name)}</td>
            <td>${f.quantity}</td>
            <td>${f.arrival_date || '—'}</td>
            <td>${f.sale_date || '—'}</td>
            <td>
                <button class="btn btn-success btn-sm me-1" onclick="openSaleModal(${f.id})">Продажа</button>
                <button class="btn btn-danger btn-sm" onclick="deleteFlower(${f.id})">Удалить</button>
            </td>
        </tr>`;
    });
}

document.getElementById('addFlowerBtn').addEventListener('click', async () => {
    const body = {
        name:         document.getElementById('flowerName').value,
        quantity:     parseInt(document.getElementById('flowerQty').value) || 0,
        arrival_date: document.getElementById('flowerArrival').value || null
    };
    if (!body.name) return;
    await fetch('/api/flowers/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
    });
    bootstrap.Modal.getInstance(document.getElementById('addFlowerModal')).hide();
    document.getElementById('flowerName').value = '';
    document.getElementById('flowerQty').value = '1';
    document.getElementById('flowerArrival').value = '';
    loadSellerFlowers();
});

window.deleteFlower = async (id) => {
    if (!confirm('Удалить запись о цветке?')) return;
    await fetch(`/api/flowers/${id}/delete`, { method: 'POST' });
    loadSellerFlowers();
};

window.openSaleModal = (id) => {
    editingFlowerId = id;
    document.getElementById('saleDate').value = new Date().toISOString().slice(0, 10);
    new bootstrap.Modal(document.getElementById('saleModal')).show();
};

document.getElementById('saveSaleBtn').addEventListener('click', async () => {
    const saleDate = document.getElementById('saleDate').value;
    if (!saleDate) return;
    await fetch(`/api/flowers/${editingFlowerId}/sale`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sale_date: saleDate })
    });
    bootstrap.Modal.getInstance(document.getElementById('saleModal')).hide();
    loadSellerFlowers();
});

function esc(s) { return (s || '').replace(/</g, '&lt;'); }
