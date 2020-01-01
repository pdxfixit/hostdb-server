$(function () {
    $(document).ready(function (e) {
        let controlForm = $('#repeatable:first'),
            firstEntry = controlForm.children('.entry:first'),
            params = window.location.search.substring(1).split("&");

        if (params.length > 0 && params[0].length > 0) {
            params.forEach(function (p) {
                let queryparam = p.split('='),
                    key = queryparam[0],
                    value = queryparam[1];

                if (key.charAt(0) == "_" && key != "_search") return;

                let newEntry = $(firstEntry.clone()).appendTo(controlForm),
                    displayText = $("a.dropdown-item[data-key='" + key + "']").html();

                newEntry.find('input').attr('name', key).val(decodeURIComponent(value));

                newEntry.find('button.dropdown-toggle').text(displayText);
                controlForm.find('.entry:not(:last) .btn-add')
                    .removeClass('btn-add').addClass('btn-remove')
                    .removeClass('btn-success').addClass('btn-danger')
                    .html('<span class="g-minus" aria-hidden="true"></span>');
                controlForm.find('.entry:last .btn-lg')
                    .removeClass('btn-remove').addClass('btn-add')
                    .removeClass('btn-danger').addClass('btn-success')
                    .html('<span class="g-plus" aria-hidden="true"></span>');
            });

            firstEntry.remove();
        }
    }).on('click', '.btn-add', function (e) {
        // create a new search field
        e.preventDefault();
        let controlForm = $('#repeatable:first'),
            currentEntry = $(this).parents('.entry:first'),
            newEntry = $(currentEntry.clone()).appendTo(controlForm);
        newEntry.find('input').attr('name', '').val('');
        newEntry.find('button.dropdown-toggle').text('Choose... ');
        newEntry.find('.catalog-button-group').remove();
        controlForm.find('.entry:not(:last) .btn-add')
            .removeClass('btn-add').addClass('btn-remove')
            .removeClass('btn-success').addClass('btn-danger')
            .html('<span class="g-minus" aria-hidden="true"></span>');
    }).on('click', '.btn-remove', function (e) {
        // remove the chosen search field
        e.preventDefault();
        $(this).parents('.entry:first').remove();
        return false;
    }).on('click', '.field-button-group .dropdown-item', function (e) {
        //
        e.preventDefault();
        let currentEntry = $(this).parents('.entry:first'),
            key = $(this).attr('data-key'),
            text = $(this).text();
        currentEntry.find('input').attr('name', key);
        currentEntry.find('.btn:first').text(text).val(text);

        if (key.charAt(0) == "_") return true;

        // get current values for the chosen field
        $.getJSON("/v0/catalog/" + key, function (data) {
            currentEntry.find('.catalog-button-group').remove();

            var items = [];
            $.each(data.catalog.sort(), function (k, v) {
                items.push('<a class="dropdown-item" href="#">' + v + '</a>');
            });

            $("<div/>", {
                "class": "btn-group catalog-button-group"
            }).appendTo(currentEntry.find('div.input-group-prepend'));

            $("<button/>", {
                "aria-expanded": "false",
                "aria-haspopup": "true",
                "class": "btn btn-outline-secondary dropdown-toggle",
                "data-toggle": "dropdown",
                "type": "button",
                text: "Catalog... "
            }).appendTo(currentEntry.find('div.catalog-button-group'));

            $("<div/>", {
                "class": "dropdown-menu",
                html: items.join("")
            }).appendTo(currentEntry.find('div.catalog-button-group'));
        });
    }).on('click', '.catalog-button-group .dropdown-item', function (e) {
        e.preventDefault();
        let currentEntry = $(this).parents('.entry:first'),
            text = $(this).text();
        currentEntry.find('input').val(text);
        currentEntry.find('.catalog-button-group .btn:first').text(text).val(text);
    }).on('click', 'button#imfeelinglucky', function (e) {
        e.preventDefault();
        let randElement = Math.floor(Math.random() * $('div.dropdown-menu a').length),
            key = $('div.dropdown-menu a').get(randElement).getAttribute('data-key');

        $.getJSON("/v0/catalog/" + key, function (data) {
            let randValue = Math.floor(Math.random() * data.catalog.length),
                value = data.catalog[randValue];

            window.location = '/?_imfeelinglucky=true&' + key + '=' + value;
        }).fail(function () {
            alert("It looks like this feature isn't available right now. Please try again later.");
        });
    });
});

function URL_add_parameters(url, paramHash) {
    let hash = {}, parser = document.createElement('a');

    parser.href = url;

    let parameters = parser.search.split(/\?|&/);

    for (let i = 0; i < parameters.length; i++) {
        if (!parameters[i])
            continue;

        let ary = parameters[i].split('=');
        hash[ary[0]] = ary[1];
    }

    Object.keys(paramHash).forEach(function (key) {
        hash[key] = paramHash[key];
    });

    let list = [];
    Object.keys(hash).forEach(function (key) {
        list.push(key + '=' + hash[key]);
    });

    parser.search = '?' + list.join('&');
    return parser.href;
}
