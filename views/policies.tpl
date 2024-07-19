<div class="p-3 row">

    <h3>Active Throttling Policies</h3>

    <table class="table" id="glob-usage">
        <thead>
            <tr>
                <th scope="col">#</th>
                <th scope="col">SrcIp</th>
                <th scope="col">Glob Group</th>
                <th scope="col">Bandwidth Class</th>
                <th scope="col">Time applied</th>
            </tr>
        </thead>
        <tbody>
            {{ range .Policies }}
            <tr>
                <th scope="row"></th>
                <td>{{ .SrcIp }}</td>
                <td>{{ .GlobGroup }}</td>
                <td>{{ .Class }}</td>
                <td>{{ .Tstamp }}</td>
            </tr>
            {{end}}

        </tbody>
    </table>

</div>