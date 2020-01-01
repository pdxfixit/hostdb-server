$(document).ready(function () {
    var table = $('#hostdb-result-table').DataTable({
        "columnDefs": [
            {
                "render": $.fn.dataTable.render.ellipsis(42),
                "targets": "_all"
            }, {
                "className": 'details-control',
                "orderable": false,
                "targets": 0,
                "width": "5%"
            }, {
                "visible": false,
                "targets": [1,5]
            }
        ],
        "info": false,
        "paging": false,
        "searching": false,
        "scrollX": true
    });
    var tree = undefined;

    // Add event listener for opening and closing details
    $('#hostdb-result-table tbody').on('click', 'td.details-control', function () {
        var tr = $(this).closest('tr');
        var row = table.row(tr);

        if (row.child.isShown()) {
            // This row is already open - close it
            row.child.hide();
            table.columns.adjust();
            $(this).removeClass('g-chevron-down').addClass('g-chevron-right');
        } else {
            // Open this row
            var fieldId = 'json-' + tr.attr('id');
            row.child('<div class="json" id="' + fieldId + '"></div>').show();
            jsonTree.create(JSON.parse(tr.attr('data-json')), document.getElementById(fieldId)).expand();
            table.columns.adjust();
            $(this).removeClass('g-chevron-right').addClass('g-chevron-down');
        }
    });
});
