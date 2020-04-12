class Messages extends Feed {
    send() {
        let btn = $("#send-message")
        if(btn.hasClass("disabled"))
            return false

        let form = $("#message-form")
        if(!form[0].reportValidity())
            return false

        btn.addClass("disabled")

        let save = form.data("id") > 0
        if(save)
            return this.save()

        return this.post()
    }
    save() {
        let form = $("#message-form")
        let wasAtBottom = this.atBottom()

        form.ajaxSubmit({
            resetForm: true,
            headers: {
                "X-Error-Type": "JSON",
            },
            success: (data) => {
                let msg = this.postLoadItem(data)
                let id = form.data("id")
                $("#message"+id).replaceWith(msg)

                msg.find("iframe.yt-video").each(prepareYtPlayer)
                CRUMINA.mediaPopups(msg)
                addYtPlayers()

                if(wasAtBottom) {
                    this.scrollToBottom()
                    msg.find(".message-content").imagesLoaded()
                        .progress(() => { this.scrollToBottom() })
                }
            },
            error: showAjaxError,
            complete: () => {
                $("#send-message").removeClass("disabled")
                this.clearForm()
            },
        })

        return false
    }
    post() {
        let uid = $("#message-uid")
        if(!uid.val())
            uid.val(Date.now())

        $("#message-form").ajaxSubmit({
            resetForm: true,
            headers: {
                "X-Error-Type": "JSON",
            },
            success: (data) => {
                uid.val("")

                let msg = this.postLoadItem(data)

                let ul = $("ul.comments-list")
                let id = msg.data("id")
                let prev = ul.find("#message" + id)
                if(prev.length)
                    prev.replaceWith(msg)
                else
                    ul.append(msg)

                msg.find("iframe.yt-video").each(prepareYtPlayer)
                CRUMINA.mediaPopups(msg)
                addYtPlayers()

                $("#messages-placeholder").remove()
                this.scrollToBottom()
                msg.find(".message-content").imagesLoaded()
                    .progress(() => { this.scrollToBottom() })
            },
            error: showAjaxError,
            complete: () => {
                $("#send-message").removeClass("disabled")
            },
        })

        return false
    }
    edit(a) {
        let msg = $(a).parents(".comment-item")
        let id = msg.data("id")
        let content = unescapeHtml(msg.data("content") + "")
        let form = $("#message-form")
        form.attr("action", "/messages/"+id)
        form.data("id", id)
        form.find("textarea").val(content)
        $("#cancel-message").toggleClass("hidden", false)
        $("#send-message").text("Сохранить")

        return false
    }
    delete(a) {
        if(!confirm("Сообщение будет удалено навсегда."))
            return false

        let msg = $(a).parents(".comment-item")
        let id = msg.data("id")

        $.ajax({
            url: "/messages/" + id,
            method: "DELETE",
            success: function() {
                msg.remove()
            },
            error: showAjaxError,
        })

        return false
    }
    clearForm() {
        let form = $("#message-form")
        let name = $("#chat-wrapper").data("name")
        form.attr("action", "/chats/" + name + "/messages")
        form.data("id", "")
        form[0].reset()
        $("#cancel-message").toggleClass("hidden", true)
        $("#send-message").text("Отправить")

        return false
    }
    atBottom() {
        let scroll = $("div.messages")
        let list = $("ul", scroll)
        return scroll.scrollTop() >= list.height() - scroll.height()
    }
    scrollToBottom() {
        let scroll = $("div.messages")
        let list = $("ul", scroll)
        scroll.scrollTop(list.height() - scroll.height())
    }
    addClickHandler(ul) {
        $("a.delete-message", ul).click((e) => { return this.delete(e.target) })
        $("a.edit-message", ul).click((e) => { return this.edit(e.target) })
    }
    readAll() {
        if(!this.unread)
            return

        $("ul.comments-list > li.un-read").removeClass("un-read")

        this.setUnread(0)

        $.ajax({
            url: "/chats/" + this.name + "/read?message=" + this.after,
            method: "PUT",
        })
    }
    check() {
        if(!this.preCheck())
            return

        let wasAtBottom = this.atBottom()

        $.ajax({
            url: "/chats/" + this.name + "/messages?after=" + this.after,
            method: "GET",
            success: (data) => {
                let ul = this.postCheck(data)
                let list = $("ul.comments-list")
                list.append(ul).children(".data-helper").remove()

                ul.find("iframe.yt-video").each(prepareYtPlayer)
                ul.each(function(){ CRUMINA.mediaPopups(this) })
                addYtPlayers()

                if(list.children().length > 0) {
                    $("#messages-placeholder").remove()
                    if(!this.before) {
                        this.setBefore(ul)
                    }
                }
                if(wasAtBottom) {
                    this.scrollToBottom()
                    ul.find(".message-content").imagesLoaded()
                        .progress(() => { this.scrollToBottom() })
                }
            },
            error: (req) => {
                let resp = JSON.parse(req.responseText)
                console.log(resp.message)
            },
            complete: () => { this.postLoadList() },
        })
    }
    loadHistory() {
        if(!this.preLoadHistory())
            return

        $.ajax({
            url: "/chats/" + this.name + "/messages?before=" + this.before,
            method: "GET",
            success: (data) => {
                let ul = this.postLoadHistory(data)
                let list = $("ul.comments-list")
                list.prepend(ul).children(".data-helper").remove()

                ul.find("iframe.yt-video").each(prepareYtPlayer)
                ul.each(function(){ CRUMINA.mediaPopups(this) })
                addYtPlayers()

                if(list.children().length > 0)
                    $("#messages-placeholder").remove()
            },
            error: (req) => {
                let resp = JSON.parse(req.responseText)
                console.log(resp.message)
            },
            complete: () => { this.postLoadList() },
        })
    }
    read(id) {
        let li = $("#message" + id)
        if(li.hasClass("un-read")) {
            li.removeClass("un-read")
            this.setUnread(this.unread - 1)
        }
    }
    update(id) {
        let old = $("#message" + id)
        if(!old.length)
            return

        $.ajax({
            url: "/messages/" + id,
            method: "GET",
            success: (data) => {
                let li = this.postLoadItem(data)
                old.replaceWith(li)
            },
            error: (req) => {
                let resp = JSON.parse(req.responseText)
                console.log(resp.message)
            },
        })
    }
    remove(id) {
        let li = $("#message" + id)
        if(li.hasClass("un-read"))
            this.setUnread(this.unread - 1)

        li.remove()
    }
    start() {
        this.name = $("#chat-wrapper").data("name")
        this.sound = new Audio("/assets/notification.mp3")
        this.check()
    }
}

$(function() {
    window.messages = new Messages()
    window.messages.start()
})

$("#send-message").click(() => { return  window.messages.send() })
$("#cancel-message").click(() => { return window.messages.clearForm() })

$("#message-form textarea").on("keydown", (e) => {
    if(e.key != "Enter")
        return

    if(e.shiftKey)
        return

    if(window.isTouchScreen)
        return

    return window.messages.send()
})

$("div.messages").scroll(function() {
    if($(this).scrollTop() < 300)
        window.messages.loadHistory()
});
