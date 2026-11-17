package main

import "github.com/jameskbb/casebook/internal/app"

func (d *Desktop) GetReviewFindings(invRef string) ([]app.ReviewFinding, error) {
	if err := d.ready(); err != nil {
		return nil, err
	}
	rows, err := d.app.GetReviewFindings(d.ctx, invRef)
	return rows, mapErr(err)
}

func (d *Desktop) ReviewStatement(invRef string, in app.ReviewStatementInput) (app.ReviewFinding, error) {
	if err := d.ready(); err != nil {
		return app.ReviewFinding{}, err
	}
	row, err := d.app.ReviewStatement(d.ctx, d.actor, invRef, in)
	return row, mapErr(err)
}
