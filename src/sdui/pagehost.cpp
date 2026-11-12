// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors
#include "pagehost.h"

#include "../engine/engineclient.h"
#include "sduiview.h"

#include <QJsonArray>
#include <QPointer>
#include <QResizeEvent>
#include <QVBoxLayout>

namespace wmp {

PageHost::PageHost(EngineClient *engine, StateFn state, QObject *parent) : QObject(parent), engine_(engine), state_(std::move(state)) {}

QString PageHost::pageOf(const SduiView *view) { return view->property("wmpPage").toString(); }

QJsonObject PageHost::errorDocument(const QString &message)
{
    QJsonObject notice{{"type", "Notice"}, {"id", "error"}, {"props", QJsonObject{{"tone", "danger"}, {"text", message}}}};
    QJsonObject root{{"type", "Page"}, {"id", "page"}, {"props", QJsonObject{{"title", "Something went wrong"}}}, {"children", QJsonArray{notice}}};
    return {{"version", 1}, {"catalog", 7}, {"page", QJsonObject{{"id", "error"}, {"kind", "custom"}, {"title", "Something went wrong"}}},
            {"layout", QJsonObject{{"breakpoint", "large"}, {"text", QJsonObject{{"body", 14}, {"title", 16}, {"caption", 12}}}, {"spacing", QJsonObject{{"s", 8}, {"m", 16}, {"l", 24}}}}},
            {"root", root}};
}

void PageHost::show(SduiView *view, const QString &page)
{
    view->setProperty("wmpPage", page);
    reload(view);
}

void PageHost::reload(SduiView *view)
{
    const QString page = pageOf(view);
    if (page.isEmpty())
        return;
    QJsonObject params{{"page", page}, {"width", view->width()}, {"height", view->height()}, {"state", state_()}};
    engine_->call(QStringLiteral("pages"), params, [this, view = QPointer<SduiView>(view), page](const EngineReply &r) {
        if (!view || pageOf(view) != page)
            return;
        const QJsonObject doc = r.ok() ? r.result.toObject() : QJsonObject();
        if (!r.ok() || !view->setDocument(doc))
            view->setDocument(errorDocument(r.ok() ? QStringLiteral("This page's document isn't one this player can draw.") : r.error));
        // A page that was made for the area it was asked at is drawn in it: ask again if the area changed since.
    });
    connect(view, &SduiView::activated, this, [this, view](const QJsonObject &a) { emit activated(view, a); }, Qt::UniqueConnection);
}

PageDialog::PageDialog(PageHost *host, const QString &page, QWidget *parent) : QDialog(parent), host_(host)
{
    setWindowTitle(QStringLiteral("WittyMediaPlayer"));
    setAttribute(Qt::WA_DeleteOnClose);
    resize(780, 700);
    auto *l = new QVBoxLayout(this);
    l->setContentsMargins(0, 0, 0, 0);
    view_ = new SduiView(this);
    l->addWidget(view_);
    resized_.setSingleShot(true);
    resized_.setInterval(150);
    connect(&resized_, &QTimer::timeout, this, [this] { host_->reload(view_); });
    connect(view_, &SduiView::documentChanged, this, [this](const QString &title) { setWindowTitle(title + QStringLiteral(" - WittyMediaPlayer")); });
    host_->show(view_, page);
}

void PageDialog::resizeEvent(QResizeEvent *e)
{
    QDialog::resizeEvent(e);
    if (e->oldSize().width() != e->size().width())
        resized_.start();
}

} // namespace wmp
