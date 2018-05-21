function logout() {
    document.cookie = 'api_token=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;'
}

function setOnline() {
    function sendRequest() {
        var req = new XMLHttpRequest()
        req.open('PUT', '/me/online', true)
        req.send()        
    }

    setInterval(sendRequest, 300000)

    sendRequest()
}

$(setOnline)

function formatDate(unix) {
    var today = new Date();
    var date = new Date(unix)
    
    if(today.getMonth() == date.getMonth()) {
        if(today.getDate() == date.getDate())
            return "Сегодня в " + date.getHours() + ":" + date.getMinutes();

        if(today.getDate() == date.getDate() + 1)
            return "Вчера в " + date.getHours() + ":" + date.getMinutes();
    }

    var str = date.getDate()

    switch (date.getMonth()) {
    case 0:
        str += " января";
        break;
    case 1:
        str += " февраля";
        break;
    case 2:
        str += " марта";
        break;
    case 3:
        str += " апреля";
        break;
    case 4:
        str += " мая";
        break;
    case 5:
        str += " июня";
        break;
    case 6:
        str += " июля";
        break;
    case 7:
        str += " августа";
        break;
    case 8:
        str += " сентябя";
        break;
    case 9:
        str += " октября";
        break;
    case 10:
        str += " ноября";
        break;
    case 11:
        str += " декабря";
        break;
    default:
        str += " " + date.getMonth();
        break;
    }

    if (today.getFullYear() !== date.getFullYear())
        str += " " + date.getFullYear();

    return str;
}

$("time").each(function() {
    var unix = $(this).datetime()
    var text = formatDate(unix)
    $(this).text(text)
})
